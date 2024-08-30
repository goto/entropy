package testbench

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/goto/entropy/cli"
	"github.com/goto/entropy/pkg/logger"
	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
	"github.com/goto/salt/dockertestx"
	"github.com/goto/salt/log"
	"github.com/ory/dockertest/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/kind/pkg/cluster"
)

var (
	UserIDHeader    = "user-id"
	zapLogger       = log.NewZap()
	TestDataPath    = ""
	TestClusterName = fmt.Sprintf("test-cluster-%s", uuid.New())
	TestNamespace   = "default"
)

func SetupTests(t *testing.T, spawnWorkers bool, setupKube bool) (context.Context, entropyv1beta1.ModuleServiceClient, entropyv1beta1.ResourceServiceClient, *cli.Config,
	*dockertest.Pool, *dockertest.Resource, *cluster.Provider, func(), func(), func()) {
	t.Helper()

	servicePort, err := getFreePort()
	if err != nil {
		t.Fatal(err)
	}

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}

	var provider *cluster.Provider
	if setupKube {
		zapLogger.Info("creating cluster")
		provider = cluster.NewProvider()
		err = provider.Create(TestClusterName)
		if err != nil {
			t.Fatal(err)
		}
	}

	zapLogger.Info("creating postgres")
	postgres, err := dockertestx.CreatePostgres(dockertestx.PostgresWithDockerPool(pool), dockertestx.PostgresWithDockertestResourceExpiry(3000))
	if err != nil {
		t.Fatal(err)
	}

	appConfig := &cli.Config{
		Service: cli.ServeConfig{
			Host: "localhost",
			Port: servicePort,
		},
		PGConnStr: postgres.GetExternalConnString(),
		Log: logger.LogConfig{
			Level: "info",
		},
		Syncer: cli.SyncerConf{
			SyncInterval:        10 * time.Second,
			RefreshInterval:     15 * time.Second,
			ExtendLockBy:        20 * time.Second,
			SyncBackoffInterval: 15 * time.Second,
			MaxRetries:          5,
		},
	}

	ctx, cancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{
		UserIDHeader: "doe@gotocompany.com",
	})))

	go func() {
		if err := cli.StartServer(ctx, *appConfig, true, spawnWorkers); err != nil {
			zapLogger.Warn(err.Error())
		}
	}()

	zapLogger.Info("creating client")
	host := fmt.Sprintf("localhost:%d", appConfig.Service.Port)
	moduleClient, cancelModuleClient, err := createModuleClient(ctx, host)
	if err != nil {
		t.Fatal(err)
	}

	resourceClient, cancelResourceClient, err := createResourceClient(ctx, host)
	if err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	parent := filepath.Dir(wd)
	TestDataPath = parent + "/testbench/test_data/"

	err = BootstrapKubernetesModule(ctx, moduleClient, TestDataPath)
	if err != nil {
		t.Fatal(err)
	}

	err = BootstrapFirehoseModule(ctx, moduleClient, TestDataPath)
	if err != nil {
		t.Fatal()
	}

	if setupKube {
		err = BootstrapKubernetesResource(ctx, resourceClient, provider, TestDataPath)
		if err != nil {
			t.Fatal()
		}
	}

	return ctx, moduleClient, resourceClient, appConfig, pool, postgres.GetResource(), provider, cancelModuleClient, cancelResourceClient, cancel
}

func SetupWorker(t *testing.T, ctx context.Context, appConfig cli.Config) {
	go func() {
		if err := cli.StartWorkers(ctx, appConfig); err != nil {
			zapLogger.Warn(err.Error())
		}
	}()
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func createResourceClient(ctx context.Context, host string) (entropyv1beta1.ResourceServiceClient, func(), error) {
	conn, err := createConnection(ctx, host)
	if err != nil {
		return nil, nil, err
	}

	cancel := func() {
		conn.Close()
	}

	client := entropyv1beta1.NewResourceServiceClient(conn)
	return client, cancel, nil
}

func createModuleClient(ctx context.Context, host string) (entropyv1beta1.ModuleServiceClient, func(), error) {
	conn, err := createConnection(ctx, host)
	if err != nil {
		return nil, nil, err
	}

	cancel := func() {
		conn.Close()
	}

	client := entropyv1beta1.NewModuleServiceClient(conn)
	return client, cancel, nil
}

func createConnection(ctx context.Context, host string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}

	return grpc.DialContext(ctx, host, opts...)
}

func GetClusterCredentials(kubeProvider *cluster.Provider, clusterName string) (string, string, string, error) {
	strConfig, err := kubeProvider.KubeConfig(clusterName, false)
	if err != nil {
		return "", "", "", err
	}

	type Cluster struct {
		Cluster struct {
			Server string `yaml:"server"`
		} `yaml:"cluster"`
	}

	type User struct {
		User struct {
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientKeyData         string `yaml:"client-key-data"`
		} `yaml:"user"`
	}

	type kubeConfig struct {
		Users   []User    `yaml:"users"`
		Cluster []Cluster `yaml:"clusters"`
	}

	var config kubeConfig
	err = yaml.Unmarshal([]byte(strConfig), &config)
	if err != nil {
		return "", "", "", err
	}

	if len(config.Users) == 0 {
		return "", "", "", errors.New("error parsing credentials")
	}

	clientCertificate, err := decodeBase64(config.Users[0].User.ClientCertificateData)
	if err != nil {
		return "", "", "", err
	}

	clientKey, err := decodeBase64(config.Users[0].User.ClientKeyData)
	if err != nil {
		return "", "", "", err
	}

	host := config.Cluster[0].Cluster.Server

	return host, clientCertificate, clientKey, nil
}

func decodeBase64(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("error decoding base64 certificate: %w", err)
	}
	return string(decoded), nil
}
