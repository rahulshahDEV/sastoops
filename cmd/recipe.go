package cmd

import (
	"fmt"

	"github.com/rahulshahDEV/sastoops/internal/recipe"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

var recipeCmd = &cobra.Command{
	Use:     "recipe",
	Aliases: []string{"recipes"},
	Short:   "ServerOps Recipes — reusable server profiles (base, production)",
}

func init() {
	recipeCmd.AddCommand(recipeListCmd, recipeShowCmd, recipeApplyCmd)
	recipeApplyCmd.Flags().StringSlice("set", nil, "override recipe params: --set key=value")
}

var recipeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available recipes",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := recipe.All()
		if err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(names)
		}
		t := ui.NewTable("RECIPE", "STEPS")
		for _, n := range names {
			r, err := recipe.Load(n)
			if err != nil {
				continue
			}
			t.Add(n, fmt.Sprintf("%d steps (%s)", len(r.Steps), r.Description))
		}
		t.Render()
		return nil
	},
}

var recipeShowCmd = &cobra.Command{
	Use:   "show <recipe>",
	Short: "Show a recipe's steps",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := recipe.Load(args[0])
		if err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(r)
		}
		ui.Section(r.Name + " v" + r.Version)
		ui.Info("%s", r.Description)
		t := ui.NewTable("#", "STEP", "MODULE", "DESCRIPTION")
		for i, s := range r.Steps {
			t.Add(fmt.Sprintf("%d", i+1), s.ID, s.Module, s.Description)
		}
		t.Render()
		return nil
	},
}

var recipeApplyCmd = &cobra.Command{
	Use:   "apply <recipe> [name]",
	Short: "Apply a recipe to a server (idempotent, safe to re-run)",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rName := args[0]
		serverName := resolveServerName(oneArg(args[1:]))
		sets, _ := cmd.Flags().GetStringSlice("set")
		overrides := map[string]string{}
		for _, kv := range sets {
			k, v, ok := cutKV(kv)
			if ok {
				overrides[k] = v
			}
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		return applyRecipe(client, serverName, rName, overrides)
	},
}

func cutKV(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
