package preflight

import "testing"

func ciSource(path, content string) DiscoverySource {
	return DiscoverySource{Path: path, Kind: "workflow", Content: content, Digest: inspectHash(content), StartLine: 1, EndLine: 1}
}
func TestStructuralWorkflowRemovalAndRestoration(t *testing.T) {
	before := []DiscoverySource{ciSource(".github/workflows/ci.yml", "name: CI\non: [push, pull_request]\njobs:\n  contract:\n    name: contract\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo check\n")}
	after := []DiscoverySource{ciSource(".github/workflows/ci.yml", "name: CI\non: push\njobs:\n  other:\n    runs-on: ubuntu-latest\n")}
	g := newDiscoveryGitHub()
	g.Requirements = append(g.Requirements, DiscoveryGitHubRequirement{ID: "required", Context: "contract", Producer: "app", AppID: 42})
	g.Results = append(g.Results, DiscoveryGitHubResult{Name: "contract", AppID: 42, AppSlug: "github-actions", Kind: "check_run"})
	changes := ExplainStructuralCI(before, after, g)
	if len(changes) != 2 {
		t.Fatalf("want removed PR trigger and uniquely mapped job: %+v", changes)
	}
	if changes[0].Before == "" || changes[0].After == "" || changes[0].BeforeLine == 0 {
		t.Fatalf("missing values/location: %+v", changes)
	}
	deleted := ExplainStructuralCI(before, nil, g)
	if len(deleted) != 1 || deleted[0].Kind != "workflow_deleted" {
		t.Fatalf("%+v", deleted)
	}
	if got := ExplainStructuralCI(after, before, g); len(got) != 0 {
		t.Fatalf("restoration retained removal: %+v", got)
	}
}
func TestStructuralCIAmbiguityNeverClaimsKnownCause(t *testing.T) {
	for _, s := range []string{
		"on: [push, pull_request]\njobs:\n  contract:\n    name: ${{ matrix.name }}\n",
		"on: &events [pull_request]\njobs:\n  contract: *job\n",
		"on: [pull_request]\njobs: {contract: {runs-on: ubuntu-latest}}\n",
	} {
		before := []DiscoverySource{ciSource(".github/workflows/a.yml", s)}
		after := []DiscoverySource{ciSource(".github/workflows/a.yml", "on: push\njobs: {}\n")}
		for _, c := range ExplainStructuralCI(before, after, newDiscoveryGitHub()) {
			if c.Verification == "verified" && c.Kind == "required_job_deleted" {
				t.Fatalf("unsupported mapping verified: %+v", c)
			}
		}
	}
	g := newDiscoveryGitHub()
	g.Requirements = append(g.Requirements, DiscoveryGitHubRequirement{ID: "required", Context: "contract", Producer: "any"})
	before := []DiscoverySource{ciSource(".github/workflows/a.yml", "on: pull_request\njobs:\n  contract:\n    runs-on: ubuntu-latest\n")}
	for _, c := range ExplainStructuralCI(before, []DiscoverySource{ciSource(before[0].Path, "on: pull_request\njobs: {}\n")}, g) {
		if c.Kind == "required_job_deleted" && c.Verification == "verified" {
			t.Fatal("unknown producer mapped")
		}
	}
}

func TestStructuralCIAlternateTriggerRepresentationsRemainUnverified(t *testing.T) {
	before := []DiscoverySource{ciSource(".github/workflows/ci.yml", "on: pull_request\njobs: {}\n")}
	for _, contents := range []string{
		"on:\n    - pull_request\njobs: {}\n",
		"on:\n    pull_request:\njobs: {}\n",
		"{on: pull_request, jobs: {}}\n",
		"on: push\non:\n  pull_request:\njobs: {}\n",
	} {
		t.Run(contents, func(t *testing.T) {
			for _, explanation := range ExplainStructuralCI(before, []DiscoverySource{ciSource(before[0].Path, contents)}, nil) {
				if explanation.Kind == "pr_trigger_removed" && explanation.Verification == "verified" {
					t.Fatalf("unsupported trigger representation fabricated removal: %+v", explanation)
				}
			}
		})
	}
}

func TestStructuralCIReplacementAndUnknownJobsDoNotImplyMissingRequiredCheck(t *testing.T) {
	g := newDiscoveryGitHub()
	g.Requirements = []DiscoveryGitHubRequirement{{ID: "required", Context: "contract", Producer: "app", AppID: 42}}
	g.Results = []DiscoveryGitHubResult{{Kind: "check_run", Name: "contract", AppID: 42, AppSlug: "github-actions"}}
	before := []DiscoverySource{ciSource(".github/workflows/ci.yml", "on: pull_request\njobs:\n  contract:\n    runs-on: ubuntu-latest\n")}
	for _, contents := range []string{
		"on: pull_request\njobs:\n  replacement:\n    name: contract\n    runs-on: ubuntu-latest\n",
		"on: pull_request\njobs:\n  replacement:\n    name: ${{ vars.JOB_NAME }}\n    runs-on: ubuntu-latest\n",
		"on: pull_request\njobs:\n  replacement:\n      name: contract\n      runs-on: ubuntu-latest\n",
	} {
		t.Run(contents, func(t *testing.T) {
			for _, explanation := range ExplainStructuralCI(before, []DiscoverySource{ciSource(before[0].Path, contents)}, g) {
				if explanation.Kind == "required_job_deleted" && explanation.Verification == "verified" {
					t.Fatalf("replacement mapping fabricated missing required job: %+v", explanation)
				}
			}
		})
	}
	after := []DiscoverySource{ciSource(before[0].Path, "on: pull_request\njobs: {}\n"), ciSource(".github/workflows/other.yml", "on: pull_request\njobs:\n  replacement:\n    name: contract\n    runs-on: ubuntu-latest\n")}
	for _, explanation := range ExplainStructuralCI(before, after, g) {
		if explanation.Kind == "required_job_deleted" {
			t.Fatalf("job moved to another workflow: %+v", explanation)
		}
	}
}

func TestStructuralCIDuplicateOrUnrecognizedJobNameIsNotMapped(t *testing.T) {
	g := newDiscoveryGitHub()
	g.Requirements = []DiscoveryGitHubRequirement{{ID: "required", Context: "contract", Producer: "app", AppID: 42}}
	g.Results = []DiscoveryGitHubResult{{Kind: "check_run", Name: "contract", AppID: 42, AppSlug: "github-actions"}}
	for _, contents := range []string{
		"on: pull_request\njobs:\n  contract:\n      name: actual-other-name\n      runs-on: ubuntu-latest\n",
		"on: pull_request\njobs:\n  contract:\n    name: other\n    name: contract\n    runs-on: ubuntu-latest\n",
		"on: pull_request\njobs:\n  contract:\n    strategy:\n      matrix: {version: [1, 2]}\n    name: contract\n    runs-on: ubuntu-latest\n",
		"on: pull_request\njobs:\n  contract:\n    uses: ./.github/workflows/reusable.yml\n    name: contract\n",
	} {
		t.Run(contents, func(t *testing.T) {
			before := []DiscoverySource{ciSource(".github/workflows/ci.yml", contents)}
			for _, explanation := range ExplainStructuralCI(before, []DiscoverySource{ciSource(before[0].Path, "on: pull_request\njobs: {}\n")}, g) {
				if explanation.Kind == "required_job_deleted" && explanation.Verification == "verified" {
					t.Fatalf("ambiguous name mapped: %+v", explanation)
				}
			}
		})
	}
}
