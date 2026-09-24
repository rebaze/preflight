package main

import rego.v1

# The wrapper supplies exactly these three documents; direct Conftest uses the
# same package and normalized facts without a second decision implementation.
facts := [doc.contents | some doc in input; doc.path == "facts.json"][0]
baseline := [doc.contents | some doc in input; doc.path == "baseline-ci.yaml"][0]
candidate := [doc.contents | some doc in input; doc.path == "candidate-ci.yaml"][0]

input_complete if {
 count([doc | some doc in input; doc.path == "facts.json"]) == 1
 count([doc | some doc in input; doc.path == "baseline-ci.yaml"]) == 1
 count([doc | some doc in input; doc.path == "candidate-ci.yaml"]) == 1
 facts.schemaVersion == 1
}
result(control,reason,msg,paths) := {"msg":msg,"metadata":{"controlId":control,"reasonCode":reason,"paths":paths}}
source_accepted(p) if {
 p.source.parsed
 p.source.scheme == "https"
 p.source.host == "registry.npmjs.org"
 not p.source.credentials
 not p.source.query
 not p.source.fragment
}
bundle_accepted(p) if {
 count(p.bundleChain) > 0
 every parent in p.bundleChain { parent.declaresChild }
 last := p.bundleChain[count(p.bundleChain)-1]
 not last.inBundle
 source_accepted(last)
 last.integrityValid
}
violation_npm contains result("npm.dependencies","invalid_policy_input","Required policy inputs are missing, duplicated or unsupported.",[]) if { not input_complete }
violation_npm contains result("npm.dependencies","unsupported_lockfile_version","The complete npm lockfile must use supported version 3.",["applications/frontend/package-lock.json"]) if {
 input_complete
 facts.npm.lockfilePresent
 facts.npm.lockfileVersion != 3
}
violation_npm contains result("npm.dependencies","override_changed",sprintf("Preserve the existing %s compatibility override at %s.",[name,version]),["applications/frontend/package.json"]) if {
 input_complete
 facts.npm.complete
 some name,version in facts.profile.requiredOverrides
 object.get(facts.npm.overrides,name,"") != version
}
violation_npm contains result("npm.dependencies","override_resolution_changed",sprintf("Installed %s must preserve compatibility version %s.",[p.name,version]),[p.path]) if {
 input_complete
 facts.npm.complete
 some p in facts.npm.packages
 version := facts.profile.requiredOverrides[p.name]
 p.version != version
}
violation_npm contains result("npm.dependencies","untrusted_registry","External packages require an HTTPS registry.npmjs.org URL without credentials, query or fragment.",[p.path]) if {
 input_complete
 facts.npm.complete
 some p in facts.npm.packages
 p.kind == "external"
 not source_accepted(p)
}
violation_npm contains result("npm.dependencies","invalid_integrity","External packages require valid SHA-512 SRI declaring a 64-byte digest.",[p.path]) if {
 input_complete
 facts.npm.complete
 some p in facts.npm.packages
 p.kind == "external"
 not p.integrityValid
}
violation_npm contains result("npm.dependencies","invalid_workspace_link","Workspace links must name a declared package inside the captured snapshot without absolute or traversing targets.",[p.path]) if {
 input_complete
 facts.npm.complete
 some p in facts.npm.packages
 p.kind == "link"
 not p.linkTargetValid
}
violation_npm contains result("npm.dependencies","invalid_bundle_chain","Bundled source inheritance requires a containing package declaration and an accepted source/integrity chain; inBundle alone grants no exemption.",[p.path]) if {
 input_complete
 facts.npm.complete
 some p in facts.npm.packages
 p.kind == "bundled"
 not bundle_accepted(p)
}
violation_npm contains result("npm.dependencies","unknown_package_record","Lockfile record is neither a root, declared workspace, valid external path nor supported link.",[p.path]) if {
 input_complete
 facts.npm.complete
 some p in facts.npm.packages
 p.kind in {"unknown","invalid"}
}

# Preserve entire semantic objects. This deliberately does not interpret shell
# commands or GitHub expressions as a proof of equivalent enforcement.
violation_ci contains result("ci.integrity","invalid_policy_input","Required policy inputs are missing, duplicated or unsupported.",[]) if { not input_complete }
violation_ci contains result("ci.integrity","workflow_missing","The required workflow is missing; intentional changes require review and re-baselining.",[".github/workflows/ci.yml"]) if {
 input_complete
 facts.scope.candidateCIMissing
}
violation_ci contains result("ci.integrity","baseline_workflow_missing","Trusted baseline workflow is missing.",[".github/workflows/ci.yml"]) if { input_complete; facts.scope.baselineCIMissing }
violation_ci contains result("ci.integrity","protected_workflow_changed",sprintf("Protected workflow object %s changed; request intentional review and re-baselining.",[key]),[".github/workflows/ci.yml"]) if {
 input_complete
 some key in facts.profile.protectedWorkflowKeys
 # Wrapping values preserves the distinction between absence and null.
 old := [value | some k,value in baseline; protected_key(k,key)]
 new := [value | some k,value in candidate; protected_key(k,key)]
 old != new
}
violation_ci contains result("ci.integrity","protected_job_changed",sprintf("Protected workflow job %s changed or is missing; request intentional review and re-baselining.",[job]),[".github/workflows/ci.yml"]) if {
 input_complete
 some job in facts.profile.requiredJobs
 old := object.get(object.get(baseline,"jobs",{}),job,null)
 new := object.get(object.get(candidate,"jobs",{}),job,null)
 old != new
}
violation_ci contains result("ci.integrity","baseline_job_missing",sprintf("Protected job %s is absent from the trusted baseline.",[job]),[".github/workflows/ci.yml"]) if {
 input_complete
 some job in facts.profile.requiredJobs
 object.get(object.get(baseline,"jobs",{}),job,null) == null
}
gate_needs contains name if { needs := baseline.jobs["build-and-test"].needs; is_array(needs); some name in needs }
gate_needs contains name if { name := baseline.jobs["build-and-test"].needs; is_string(name) }
violation_ci contains result("ci.integrity","gate_dependency_missing",sprintf("Baseline gate dependency %s must remain present; request intentional review and re-baselining.",[name]),[".github/workflows/ci.yml"]) if {
 input_complete
 some name in gate_needs
 object.get(object.get(candidate,"jobs",{}),name,null) == null
}
violation_tests contains result("eer.tests","invalid_policy_input","Required policy inputs are missing, duplicated or unsupported.",[]) if { not input_complete }
violation_tests contains result("eer.tests",facts.tests.reasonCode,facts.tests.message,[]) if { input_complete; facts.tests.status == "fail" }
violation_tests contains result("eer.tests","empty_test_evidence","Passing test evidence must record positive executed tests without failures, errors or skips.",[]) if {
 input_complete
 facts.tests.status == "pass"
 not tests_positive
}
tests_positive if { facts.tests.tests > 0; facts.tests.failures == 0; facts.tests.errors == 0; facts.tests.skipped == 0; facts.tests.exitCode == 0; count(facts.tests.files)>0 }
warn contains result("eer.tests","test_implementation_changed","Test implementation changed; a passing modified test does not independently establish unchanged intent.",facts.scope.changedTestPaths) if {
 input_complete
 count(facts.scope.changedTestPaths)>0
}

# Pinned Conftest uses YAML 1.1, whose parser resolves an unquoted on key as
# boolean true then serializes it as "true". Normalize this parser artifact so
# GitHub's quoted and unquoted trigger forms compare as the same semantic key.
protected_key(actual,expected) if { actual == expected }
protected_key(actual,expected) if { expected == "on"; actual == "true" }
gate_needs_valid if { is_array(baseline.jobs["build-and-test"].needs); every name in baseline.jobs["build-and-test"].needs { is_string(name); name != "" } }
gate_needs_valid if { is_string(baseline.jobs["build-and-test"].needs); baseline.jobs["build-and-test"].needs != "" }
violation_ci contains result("ci.integrity","invalid_gate_dependencies","The baseline gate needs field must be a job name or array of job names.",[".github/workflows/ci.yml"]) if { input_complete; not gate_needs_valid }
violation_ci contains result("ci.integrity","ambiguous_trigger_key","Workflow has both quoted and unquoted trigger keys; request intentional review and re-baselining.",[".github/workflows/ci.yml"]) if {
 input_complete
 triggers := [value | some key,value in candidate; protected_key(key,"on")]
 count(triggers)>1
}
