#!/usr/bin/env python3
"""Disposable public-CLI change/consequence/restoration demo; no project execution."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('cli',type=Path)
a=p.parse_args();cli=a.cli.resolve()
root=Path(tempfile.mkdtemp(prefix='preflight-comparison-'));repo=root/'fixture';repo.mkdir();state=root/'observations';state.mkdir(mode=0o700)
def git(*args):
 subprocess.run(['git','-C',str(repo),'-c','commit.gpgsign=false','-c','core.hooksPath=/dev/null',*args],check=True,capture_output=True)
git('init','-b','main');git('config','user.name','Synthetic');git('config','user.email','synthetic@example.invalid')
workflow=repo/'.github/workflows/ci.yml';workflow.parent.mkdir(parents=True)
original='name: Contract\non: [push, pull_request]\njobs:\n  contract:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo synthetic\n'
workflow.write_text(original);(repo/'CONTRIBUTING.md').write_text('Preserve existing response fields.\n')
git('add','.');git('commit','-m','synthetic fixture')
def inspect(name,previous=None):
 cmd=[str(cli),'inspect','--repo',str(repo),'--format','json','--output',str(state/(name+'.json'))]
 if previous:cmd+=['--compare',str(state/(previous+'.json'))]
 result=subprocess.run(cmd,capture_output=True,text=True)
 if result.returncode not in (0,2):raise RuntimeError(result.stderr+result.stdout)
 return json.loads(result.stdout)
before=inspect('before')
workflow.write_text(original.replace('[push, pull_request]','push'))
changed=inspect('changed','before')
assert changed['comparison']['previousEvidenceStale']
assert any(v['kind']=='pr_trigger_removed' for v in changed['comparison']['structural'])
workflow.write_text(original)
restored=inspect('restored','changed')
assert any(v['kind']=='pr_trigger_removed' for v in restored['comparison']['resolvedStructural'])
assert (state/'before.json').exists() and (state/'changed.json').exists()
print(json.dumps({'root':str(root),'changed':[v['kind'] for v in changed['comparison']['structural']], 'restored':[v['kind'] for v in restored['comparison']['resolvedStructural']], 'stale':changed['comparison']['previousEvidenceStale'],'executedProjectChecks':False,'remoteFailureCause':'not established by source edits; see comparison regression for observed failing evidence'}))
