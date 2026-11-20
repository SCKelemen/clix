// Package projects implements "gcloud projects": a table, a description,
// a long-running create that reports progress, and labels updated through
// a group of flags.
package projects

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// Project is what "projects list" and "projects describe" print. Its JSON
// names are the fields of --format json and the columns of the table.
type Project struct {
	ID     string            `json:"projectId"`
	Name   string            `json:"name"`
	Number int64             `json:"projectNumber"`
	State  string            `json:"lifecycleState"`
	Labels map[string]string `json:"labels,omitempty"`
}

// sampleProjects stands in for the Resource Manager API.
var sampleProjects = []Project{
	{ID: "demo-prod", Name: "Demo (production)", Number: 104723940012, State: "ACTIVE", Labels: map[string]string{"env": "prod", "team": "web"}},
	{ID: "demo-staging", Name: "Demo (staging)", Number: 104723940013, State: "ACTIVE", Labels: map[string]string{"env": "staging", "team": "web"}},
	{ID: "sandbox-7731", Name: "Sandbox", Number: 381200045121, State: "ACTIVE"},
}

// OperationStep is how long each step of the fake create operation takes.
// Tests set it to zero.
var OperationStep = 600 * time.Millisecond

// NewCommand returns the "projects" group. project is the value of the
// global --project flag, which commands use when no project is named.
func NewCommand(project *string) *clix.Command {
	return clix.NewGroup("projects", "Create and manage project access policies",
		newListCommand(),
		newDescribeCommand(project),
		newCreateCommand(),
		newUpdateCommand(project),
	)
}

func newListCommand() *clix.Command {
	list := clix.NewCommand("list")
	list.Short = "List projects accessible by the active account"
	list.Run = func(ctx *clix.Context) error {
		// An aligned table on a terminal, tab-separated values in a pipe,
		// or JSON or YAML with --format.
		return format.WriteWith(ctx, sampleProjects, clix.FormatOptions{
			Columns: []string{"projectId", "name", "projectNumber"},
		})
	}
	return list
}

func newDescribeCommand(project *string) *clix.Command {
	describe := clix.NewCommand("describe")
	describe.Short = "Show metadata for a project"
	var id string
	describe.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "project-id",
			Usage:      "Project to describe (default: the --project property)",
			Positional: true,
		},
		Value: &id,
	})
	describe.Run = func(ctx *clix.Context) error {
		p, err := lookup(firstNonEmpty(id, *project))
		if err != nil {
			return err
		}
		return format.Write(ctx, p)
	}
	return describe
}

func newCreateCommand() *clix.Command {
	create := clix.NewCommand("create")
	create.Short = "Create a new project"
	var id, name string
	create.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "project-id",
			Usage:      "ID for the project you want to create",
			Required:   true,
			Prompt:     "Project ID",
			Positional: true,
		},
		Value: &id,
	})
	create.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "name",
			Usage: "Name for the project (default: the project ID)",
		},
		Value: &name,
	})
	create.Run = func(ctx *clix.Context) error {
		// The indicator writes to stderr: a spinner on a terminal, one
		// plain line otherwise. The result goes to stdout.
		p := ctx.Progress("Creating project " + id)
		steps := []string{"checking the project ID", "provisioning", "enabling services", "applying policies"}
		for i, step := range steps {
			p.Update(float64(i)/float64(len(steps)), step)
			if err := wait(ctx, OperationStep); err != nil {
				return err // Run ends the indicator with "cancelled"
			}
		}
		p.Done("done")
		return format.Write(ctx, Project{ID: id, Name: firstNonEmpty(name, id), Number: 512000000001, State: "ACTIVE"})
	}
	return create
}

func newUpdateCommand(project *string) *clix.Command {
	update := clix.NewCommand("update")
	update.Short = "Update the labels of a project"
	var (
		id                string
		updates, removals []string
		clearAll          bool
	)
	update.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "project-id",
			Usage:      "Project to update (default: the --project property)",
			Positional: true,
		},
		Value: &id,
	})
	update.Flags.StringSliceVar(clix.StringSliceVarOptions{
		FlagOptions: clix.FlagOptions{Name: "update-labels", Usage: "Labels to add or change, as KEY=VALUE pairs"},
		Value:       &updates,
	})
	update.Flags.StringSliceVar(clix.StringSliceVarOptions{
		FlagOptions: clix.FlagOptions{Name: "remove-labels", Usage: "Label keys to remove"},
		Value:       &removals,
	})
	update.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{Name: "clear-labels", Usage: "Remove all labels"},
		Value:       &clearAll,
	})

	// The label trio, as gcloud has it: --clear-labels and --remove-labels
	// exclude each other, and at least one of the three is needed. Run
	// checks both rules before the handler runs and returns a
	// *clix.FlagGroupError (exit status 2); help shows them in the usage
	// line.
	update.Flags.MarkMutuallyExclusive("clear-labels", "remove-labels")
	update.Flags.MarkOneRequired("update-labels", "remove-labels", "clear-labels")

	update.Run = func(ctx *clix.Context) error {
		p, err := lookup(firstNonEmpty(id, *project))
		if err != nil {
			return err
		}
		labels := map[string]string{}
		if !clearAll {
			maps.Copy(labels, p.Labels)
		}
		for _, k := range removals {
			delete(labels, k)
		}
		for _, kv := range updates {
			k, v, ok := strings.Cut(kv, "=")
			if !ok || k == "" {
				return clix.UsageErrorf("--update-labels: %q is not KEY=VALUE", kv)
			}
			labels[k] = v
		}
		_, err = fmt.Fprintf(ctx.App.Out, "Updated labels of [%s]: %s\n", p.ID, formatLabels(labels))
		return err
	}
	return update
}

// lookup returns the sample project with the given ID.
func lookup(id string) (Project, error) {
	if id == "" {
		return Project{}, clix.UsageErrorf("no project: name one, pass --project, or run: gcloud config set project PROJECT_ID")
	}
	for _, p := range sampleProjects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("project [%s] not found or permission denied", id)
}

// wait sleeps for d, or until ctx is done.
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "(none)"
	}
	pairs := make([]string, 0, len(labels))
	for _, k := range slices.Sorted(maps.Keys(labels)) {
		pairs = append(pairs, k+"="+labels[k])
	}
	return strings.Join(pairs, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
