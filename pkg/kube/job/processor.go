package job

import (
	"context"
	"fmt"

	v1 "k8s.io/client-go/kubernetes/typed/batch/v1"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type Processor struct {
	Job              *Job
	Client           v1.JobInterface
	watch            watch.Interface
	JobDeleteOptions metav1.DeleteOptions
}

func (jp *Processor) GetWatch() watch.Interface {
	return jp.watch
}

type StatusType int

const (
	Invalid StatusType = iota
	Success
	Failed
	Running
	Ready
	Dummy
)

type Status struct {
	Status StatusType
	Err    error
}

var deletionPolicy = metav1.DeletePropagationForeground

func NewProcessor(job *Job, client v1.JobInterface) *Processor {
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: &deletionPolicy}
	return &Processor{Job: job, Client: client, JobDeleteOptions: deleteOptions}
}

func (jp *Processor) SubmitJob() error {
	_, err := jp.Client.Create(context.Background(), jp.Job.Template(), metav1.CreateOptions{})
	return err
}

func (jp *Processor) CreateWatch() error {
	w, err := jp.Client.Watch(context.Background(), jp.Job.WatchOptions())
	jp.watch = w
	return err
}

func (jp *Processor) GetStatus() Status {
	job, err := jp.Client.Get(context.Background(), jp.Job.Name, metav1.GetOptions{})
	status := Status{}
	if err != nil {
		status.Status = Invalid
		status.Err = err
		return status
	}
	if *job.Status.Ready >= 1 {
		status.Status = Ready
		return status
	}
	if job.Status.Active >= 1 {
		status.Status = Running
		return status
	}
	if job.Status.Succeeded >= 1 {
		status.Status = Success
		return status
	} else {
		zap.L().Error(fmt.Sprintf("JOB FAILED %v\n", job))
		status.Status = Failed
		return status
	}
}

func (jp *Processor) DeleteJob() error {
	return jp.Client.Delete(context.Background(), jp.Job.Name, jp.JobDeleteOptions)
}

func (jp *Processor) WatchCompletion(exitChan chan Status) {
	if jp.GetWatch() == nil {
		exitChan <- Status{
			Status: Invalid,
			Err:    fmt.Errorf("watcher Object is not initilised"),
		}
		return
	}
	for {
		select {
		case _, ok := <-jp.GetWatch().ResultChan():
			if !ok {
				exitChan <- Status{Status: Dummy}
				return
			}
		}
	}
}
