package preflight

import (
	"regexp"
	"sort"
	"strings"
)

type DiscoveryCIExplanation struct {
	Kind          string `json:"kind"`
	Path          string `json:"path"`
	BeforeLine    int    `json:"beforeLine"`
	AfterLine     int    `json:"afterLine"`
	Before        string `json:"before"`
	After         string `json:"after"`
	RequirementID string `json:"requirementId"`
	Verification  string `json:"verification"`
	Detail        string `json:"detail"`
}
type ciJob struct {
	id, name string
	line     int
	literal  bool
}
type ciShape struct {
	pr                      bool
	triggerLine             int
	triggerKnown, jobsKnown bool
	jobs                    []ciJob
}

var ciKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// This recognizes a deliberately small literal YAML subset. It never evaluates
// expressions, commands, anchors, aliases, tags, reusable workflows or matrices.
func inspectCIShape(content string) ciShape {
	s := ciShape{jobs: []ciJob{}, jobsKnown: true}
	section := ""
	onSeen, jobsSeen := false, false
	jobIndent := 2
	current := -1
	onBlock, onChildSeen, jobsBlock := false, false, false
	properties := map[string]bool{}
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(raw, "\t") || strings.Contains(line, "&") || strings.HasPrefix(line, "<<:") || strings.HasPrefix(line, "!") || strings.Contains(line, ": *") {
			s.triggerKnown = false
			s.jobsKnown = false
			return s
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		key, value, ok := strings.Cut(line, ":")
		key = strings.Trim(key, "\"'")
		value = strings.TrimSpace(value)
		if indent == 0 {
			if !ok || !ciKey.MatchString(key) {
				s.triggerKnown, s.jobsKnown = false, false
				return s
			}
			section = key
			current = -1
			switch key {
			case "on":
				if onSeen {
					s.triggerKnown = false
					s.jobsKnown = false
					return s
				}
				onSeen = true
				s.triggerLine = i + 1
				if value == "" {
					s.triggerKnown = true
					onBlock = true
					continue
				}
				if strings.ContainsAny(value, "{}$|>!&*") {
					s.triggerKnown = false
					continue
				}
				value = strings.Trim(value, "[]")
				s.triggerKnown = true
				for _, event := range strings.Split(value, ",") {
					event = strings.Trim(strings.TrimSpace(event), "\"'")
					if !ciKey.MatchString(event) {
						s.triggerKnown = false
					}
					if event == "pull_request" {
						s.pr = true
					}
				}
			case "jobs":
				if jobsSeen {
					s.jobsKnown = false
					return s
				}
				jobsSeen = true
				jobsBlock = value == ""
				if value != "" && value != "{}" {
					s.jobsKnown = false
				}
			}
			continue
		}
		if section == "on" && s.triggerKnown {
			if !onBlock || !onChildSeen && indent != 2 || indent < 2 {
				s.triggerKnown = false
				continue
			}
			if indent == 2 {
				onChildSeen = true
				if !ok || !ciKey.MatchString(key) {
					s.triggerKnown = false
					continue
				}
				if key == "pull_request" {
					s.pr = true
				}
			}
		}
		if section != "jobs" {
			continue
		}
		if !jobsBlock || indent < jobIndent || indent%2 != 0 {
			s.jobsKnown = false
			continue
		}
		if indent == jobIndent {
			if !ok || !ciKey.MatchString(key) || value != "" {
				s.jobsKnown = false
				continue
			}
			for _, j := range s.jobs {
				if j.id == key {
					s.jobsKnown = false
				}
			}
			s.jobs = append(s.jobs, ciJob{id: key, name: key, line: i + 1, literal: true})
			current = len(s.jobs) - 1
			properties = map[string]bool{}
		} else if current >= 0 && indent == jobIndent+2 {
			if !ok || !ciKey.MatchString(key) || properties[key] {
				s.jobsKnown = false
				continue
			}
			properties[key] = true
			if key == "name" {
				name, literal := ciLiteralName(value)
				s.jobs[current].name = name
				s.jobs[current].literal = s.jobs[current].literal && literal
			}
			if key == "strategy" || key == "uses" {
				s.jobs[current].literal = false
			}
		} else if current < 0 || len(properties) == 0 {
			s.jobsKnown = false
		}
	}
	if !onSeen {
		s.triggerKnown = true
	}
	if !jobsSeen {
		s.jobsKnown = false
	}
	return s
}

func ciLiteralName(value string) (string, bool) {
	if value == "" {
		return value, false
	}
	if value[0] == '\'' || value[0] == '"' {
		quote := value[0]
		if len(value) < 2 || value[len(value)-1] != quote {
			return value, false
		}
		value = value[1 : len(value)-1]
		if strings.ContainsRune(value, rune(quote)) || strings.ContainsRune(value, '\\') {
			return value, false
		}
	}
	return value, value != "" && !strings.ContainsAny(value, "${}[]&*!|>:#")
}

// ExplainStructuralCI records observed removals, not policy verdicts or the
// origin of a failing test. Required-job mapping needs an observed Actions app,
// a configured app-bound requirement and one literal job across all workflows.
func ExplainStructuralCI(before, after []DiscoverySource, g *DiscoveryGitHub) []DiscoveryCIExplanation {
	out := []DiscoveryCIExplanation{}
	old, newer := map[string]DiscoverySource{}, map[string]DiscoverySource{}
	for _, s := range before {
		if s.Kind == "workflow" {
			old[s.Path] = s
		}
	}
	for _, s := range after {
		if s.Kind == "workflow" {
			newer[s.Path] = s
		}
	}
	counts := map[string]int{}
	remainingNames := map[string]int{}
	allJobsKnown := true
	for _, s := range old {
		shape := inspectCIShape(s.Content)
		if !shape.jobsKnown {
			allJobsKnown = false
		}
		for _, j := range shape.jobs {
			if j.literal {
				counts[j.name]++
			} else {
				allJobsKnown = false
			}
		}
	}
	for _, s := range newer {
		shape := inspectCIShape(s.Content)
		if !shape.jobsKnown {
			allJobsKnown = false
		}
		for _, j := range shape.jobs {
			if j.literal {
				remainingNames[j.name]++
			} else {
				allJobsKnown = false
			}
		}
	}
	paths := []string{}
	for p := range old {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		a := old[p]
		b, exists := newer[p]
		if !exists {
			out = append(out, DiscoveryCIExplanation{Kind: "workflow_deleted", Path: p, BeforeLine: a.StartLine, Before: a.Digest, After: "absent", Verification: "verified", Detail: "Previously observed workflow is absent from the current complete source inventory; it cannot provide its former workflow configuration. This does not establish the cause of a test failure."})
			continue
		}
		if a.Digest == b.Digest {
			continue
		}
		x, y := inspectCIShape(a.Content), inspectCIShape(b.Content)
		if x.triggerKnown && y.triggerKnown && x.pr && !y.pr {
			out = append(out, DiscoveryCIExplanation{Kind: "pr_trigger_removed", Path: p, BeforeLine: x.triggerLine, AfterLine: y.triggerLine, Before: "pull_request present", After: "pull_request absent", Verification: "verified", Detail: "Literal pull_request trigger removed; pull_request_target or other events have different semantics. No workflow execution was performed."})
		}
		if !x.triggerKnown || !y.triggerKnown || !x.jobsKnown || !y.jobsKnown {
			out = append(out, DiscoveryCIExplanation{Kind: "unsupported_structure", Path: p, BeforeLine: 1, AfterLine: 1, Before: a.Digest, After: b.Digest, Verification: "unverified", Detail: "Unsupported or ambiguous YAML structure requires review; structural semantics were not evaluated."})
		}
		if g == nil || !allJobsKnown || !x.jobsKnown || !y.jobsKnown {
			continue
		}
		remaining := map[string]bool{}
		for _, j := range y.jobs {
			remaining[j.id] = true
		}
		for _, j := range x.jobs {
			if remaining[j.id] || !j.literal || counts[j.name] != 1 || remainingNames[j.name] != 0 {
				continue
			}
			for _, req := range g.Requirements {
				if req.Context != j.name || req.Producer != "app" || req.AppID <= 0 {
					continue
				}
				actions := false
				for _, r := range g.Results {
					if r.Kind == "check_run" && r.Name == req.Context && r.AppID == req.AppID && r.AppSlug == "github-actions" {
						actions = true
					}
				}
				if !actions {
					continue
				}
				out = append(out, DiscoveryCIExplanation{Kind: "required_job_deleted", Path: p, BeforeLine: j.line, AfterLine: 1, Before: "job " + j.id + " (" + j.name + ")", After: "absent", RequirementID: req.ID, Verification: "verified", Detail: "Deleted literal job uniquely matches the configured required check and observed GitHub Actions producer. This explains missing configuration, not when or why a remote test failed."})
			}
		}
	}
	return out
}
