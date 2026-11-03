#!/usr/bin/env python3
"""Collect reproducible Actions timing evidence using gh (no extra dependencies)."""
import argparse
import concurrent.futures
import datetime
import json
from pathlib import Path
import statistics
import subprocess


def gh(*args):
    return json.loads(subprocess.check_output(['gh', *args], text=True))


def seconds(start, end):
    if not start or not end:
        return 0
    return max(0, (datetime.datetime.fromisoformat(end.replace('Z', '+00:00')) -
                   datetime.datetime.fromisoformat(start.replace('Z', '+00:00'))).total_seconds())


def is_setup(name):
    return any(token in name.lower() for token in (
        'set up job', 'checkout', 'setup-', 'setup pnpm', 'initialize containers',
        'install --frozen-lockfile', 'post ', 'complete job', 'stop containers'))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--branch')
    parser.add_argument('--limit', type=int, default=50)
    parser.add_argument('--runs-file', type=Path)
    parser.add_argument('--jobs-dir', type=Path)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    repo = gh('repo', 'view', '--json', 'nameWithOwner')['nameWithOwner']
    runs = json.loads(args.runs_file.read_text()) if args.runs_file else gh(
        'run', 'list', '--limit', str(args.limit), '--json',
        'databaseId,name,headBranch,event,status,conclusion,createdAt,startedAt,updatedAt,url',
        *(['--branch', args.branch] if args.branch else []))

    def collect(run):
        run_id = run['databaseId']
        jobs = (json.loads((args.jobs_dir / f'{run_id}.json').read_text()) if args.jobs_dir
                else gh('api', f'repos/{repo}/actions/runs/{run_id}/jobs?per_page=100'))
        if jobs['total_count'] > 100:
            raise RuntimeError('Pagination required for more than 100 jobs')
        return {'run': run, 'jobs': jobs['jobs']}

    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as executor:
        evidence = list(executor.map(collect, runs))
    (args.output / 'raw.json').write_text(json.dumps(evidence, indent=2) + '\n')
    rows = []
    for item in evidence:
        for job in item['jobs']:
            total = seconds(job['started_at'], job['completed_at'])
            execution = sum(seconds(s['started_at'], s['completed_at'])
                            for s in job['steps'] if not is_setup(s['name']))
            rows.append({'run': item['run']['databaseId'], 'workflow': item['run']['name'],
                         'job': job['name'], 'conclusion': job['conclusion'],
                         'queue': seconds(item['run']['createdAt'], job['started_at']),
                         'setup': max(0, total - execution), 'execution': execution})
    (args.output / 'jobs.json').write_text(json.dumps(rows, indent=2) + '\n')
    report = ['# Actions timings', '',
              'Seconds; queue = workflow creation to job start (includes dependency wait).',
              'Setup includes checkout, tool/dependency setup, services, cleanup and unaccounted overhead.',
              'Execution includes build, tests, lint and security analysis; composite actions may include internal setup.',
              'Medians below include successful jobs only; raw evidence retains failures/cancellations.', '',
              '| Job | Samples | Queue | Setup | Execution |', '|---|---:|---:|---:|---:|']
    for name in sorted({row['job'] for row in rows}):
        samples = [r for r in rows if r['job'] == name and r['conclusion'] == 'success']
        if samples:
            values = [statistics.median(r[k] for r in samples) for k in ['queue', 'setup', 'execution']]
            report.append(f'| {name} | {len(samples)} | ' + ' | '.join(f'{v:g}' for v in values) + ' |')
    (args.output / 'summary.md').write_text('\n'.join(report) + '\n')
    print('\n'.join(report))


if __name__ == '__main__':
    main()
