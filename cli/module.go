package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/MakeNowJust/heredoc"
	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
	"github.com/goto/salt/printer"
	"github.com/spf13/cobra"
)

func cmdModuleCreate() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new module",
		Example: heredoc.Doc(`
			$ entropy module create -f module.yaml
			$ entropy module create --file module.yaml
		`),
		Annotations: map[string]string{
			"module": "core",
		},
	}

	cmd.RunE = handleErr(func(cmd *cobra.Command, args []string) error {
		var reqBody entropyv1beta1.Module

		if err := parseFile(filePath, &reqBody); err != nil {
			return err
		}

		if err := reqBody.ValidateAll(); err != nil {
			return err
		}

		client, cancel, err := createModuleServiceClient(cmd)
		if err != nil {
			return err
		}
		defer cancel()

		req := &entropyv1beta1.CreateModuleRequest{
			Module: &reqBody,
		}

		spinner := printer.Spin("Creating resource...")
		defer spinner.Stop()
		mod, err := client.CreateModule(cmd.Context(), req)
		if err != nil {
			return err
		}
		spinner.Stop()

		module := mod.GetModule()
		return Display(cmd, module, func(w io.Writer, v any) error {
			_, _ = fmt.Fprintf(w, "Module created with URN '%s'.\n", module.Urn)
			_, _ = fmt.Fprintln(w, "Use 'entropy module view <urn>' to view module.")
			return nil
		})
	})

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the module body file")
	cmd.MarkFlagRequired("file")

	return cmd
}

func cmdModuleUpdate() *cobra.Command {
	var filePath, urn string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a module",
		Example: heredoc.Doc(`
			$ entropy module update -u orn:entropy:module:test-project:test-name -f module.yaml
			$ entropy module update -urn orn:entropy:module:test-project:test-name -file module.yaml
		`),
		Annotations: map[string]string{
			"module": "core",
		},
	}

	cmd.RunE = handleErr(func(cmd *cobra.Command, args []string) error {
		var reqBody entropyv1beta1.Module

		if err := parseFile(filePath, &reqBody); err != nil {
			return err
		}

		if err := reqBody.ValidateAll(); err != nil {
			return err
		}

		client, cancel, err := createModuleServiceClient(cmd)
		if err != nil {
			return err
		}
		defer cancel()

		spinner := printer.Spin("Updating module...")
		defer spinner.Stop()
		req := &entropyv1beta1.UpdateModuleRequest{
			Urn:     urn,
			Configs: reqBody.Configs,
		}
		spinner.Stop()

		mod, err := client.UpdateModule(cmd.Context(), req)
		if err != nil {
			return err
		}

		module := mod.GetModule()
		return Display(cmd, module, func(w io.Writer, v any) error {
			_, _ = fmt.Fprintf(w, "Module updated with URN '%s'.\n", module.Urn)
			_, _ = fmt.Fprintln(w, "Use 'entropy module view <urn>' to view module.")
			return nil
		})
	})

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the module body file")
	cmd.MarkFlagRequired("file")
	cmd.Flags().StringVarP(&urn, "urn", "u", "", "URN of the module to update")
	cmd.MarkFlagRequired("urn")

	return cmd
}

func cmdModuleView() *cobra.Command {
	var urn string
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View a module by URN",
		Example: heredoc.Doc(`
			$ entropy module view -u orn:entropy:module:test-project:test-name
			$ entropy module view --urn orn:entropy:module:test-project:test-name
		`),
		Annotations: map[string]string{
			"module": "core",
		},
	}

	cmd.RunE = handleErr(func(cmd *cobra.Command, args []string) error {
		client, cancel, err := createModuleServiceClient(cmd)
		if err != nil {
			return err
		}
		defer cancel()

		spinner := printer.Spin("Fetching module...")
		defer spinner.Stop()
		req := &entropyv1beta1.GetModuleRequest{
			Urn: urn,
		}
		spinner.Stop()

		mod, err := client.GetModule(cmd.Context(), req)
		if err != nil {
			return err
		}

		module := mod.GetModule()
		return Display(cmd, module, func(w io.Writer, v any) error {
			printer.Table(os.Stdout, [][]string{
				{"URN", "NAME", "PROJECT", "Config"},
				{module.Urn, module.Name, module.Project, module.Configs.GetStringValue()},
			})
			return nil
		})
	})

	cmd.Flags().StringVarP(&urn, "urn", "u", "", "URN of the module to view")
	cmd.MarkFlagRequired("urn")

	return cmd
}
