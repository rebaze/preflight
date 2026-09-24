package main
import rego.v1
fixture_profile := {"requiredOverrides": {"brace-expansion":"5.0.9","js-yaml":"4.3.1"},"requiredJobs":["build-and-test"],"protectedWorkflowKeys":["on","permissions"]}
fixture_npm := {"complete":true,"lockfilePresent":true,"lockfileVersion":3,"overrides":{"brace-expansion":"5.0.9","js-yaml":"4.3.1"},"packages":[]}
fixture_facts := {"schemaVersion":1,"profile":fixture_profile,"tests":{"status":"not_applicable"},"npm":fixture_npm,"scope":{"baselineCIMissing":false,"candidateCIMissing":false,"changedTestPaths":[]}}
fixture_ci := {"on":{"pull_request":{}},"jobs":{"build-and-test":{"needs":[]}}}
fixture_input(f,baseline,candidate) := [{"path":"facts.json","contents":f},{"path":"baseline-ci.yaml","contents":baseline},{"path":"candidate-ci.yaml","contents":candidate}]
test_gate_deletion if {
 results := violation_ci with input as fixture_input(fixture_facts,fixture_ci,{"on":{"pull_request":{}},"jobs":{}})
 some item in results
 item.metadata.controlId == "ci.integrity"
}
test_untrusted_registry if {
 bad := {"path":"node_modules/example","kind":"external","name":"example","version":"1.0.0","source":{"parsed":true,"scheme":"https","host":"example.invalid","credentials":false,"query":false,"fragment":false},"integrityValid":true}
 npm := object.union(fixture_npm,{"packages":[bad]})
 facts := object.union(fixture_facts,{"npm":npm})
 results := violation_npm with input as fixture_input(facts,fixture_ci,fixture_ci)
 some item in results
 item.metadata.controlId == "npm.dependencies"
 item.metadata.reasonCode == "untrusted_registry"
}
