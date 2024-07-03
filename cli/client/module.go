package client

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
	cmd := &cobra.Command{
		Use:   "create <file>",
		Short: "Create a new module",
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			$ entropy module create module.yaml
		`),
		Annotations: map[string]string{
			"module": "core",
		},
	}

	cmd.RunE = handleErr(func(cmd *cobra.Command, args []string) error {
		var reqBody entropyv1beta1.Module

		if err := parseFile(args[0], &reqBody); err != nil {
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
			_, _ = fmt.Fprintln(w, "Use 'entropy module get <urn>' to view module.")
			return nil
		})
	})

	return cmd
}

func cmdModuleUpdate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <urn> <file>",
		Short: "Update a module",
		Args:  cobra.ExactArgs(2),
		Example: heredoc.Doc(`
			$ entropy module update orn:entropy:module:test-project:test-name module.yaml
		`),
		Annotations: map[string]string{
			"module": "core",
		},
	}

	cmd.RunE = handleErr(func(cmd *cobra.Command, args []string) error {
		var reqBody entropyv1beta1.Module

		if err := parseFile(args[1], &reqBody); err != nil {
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
			Urn:     args[0],
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
			_, _ = fmt.Fprintln(w, "Use 'entropy module get <urn>' to view module.")
			return nil
		})
	})

	return cmd
}

func cmdModuleView() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <urn>",
		Short: "View a module by URN",
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			$ entropy module get orn:entropy:module:test-project:test-name
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
			Urn: args[0],
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

	return cmd
}
