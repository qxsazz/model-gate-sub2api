#!/usr/bin/env python3
"""Report upstream versions without modifying Git refs or application data."""
import argparse
import json
import re
from datetime import datetime
from pathlib import Path


def release_report(release: dict, current: str) -> str:
    tag = release.get('tag_name', '')
    upstream = re.fullmatch(r'v(\d+)\.(\d+)\.(\d+)', tag)
    local = re.fullmatch(r'v?(\d+)\.(\d+)\.(\d+)(?:[-+][A-Za-z0-9.-]+)?', current.strip())
    if not upstream or not local or release.get('prerelease') or release.get('draft'):
        raise ValueError('Expected a stable upstream release and a semantic main VERSION')
    link = 'https://github.com/Wei-Shaw/sub2api/releases/tag/' + tag
    if release.get('html_url', link) != link:
        raise ValueError('Unexpected upstream release URL')
    newer = tuple(map(int, upstream.groups())) > tuple(map(int, local.groups()))
    status = 'New upstream version available' if newer else 'No newer upstream version'
    published = release.get('published_at')
    published = datetime.fromisoformat(published.replace('Z', '+00:00')).isoformat() if published else 'Not provided'
    return (
        '# Upstream release information\n\n'
        f'- Main VERSION: `{current.strip()}`\n'
        f'- Latest stable upstream: `{tag}`\n'
        f'- Published: {published}\n'
        f'- Release notes: [{tag}]({link})\n\n'
        f'**{status}.**\n\n'
        'Information only. No merge, push, pull request, or deployment is performed.\n'
        'Version comparison uses the base semantic version, not Git ancestry or compatibility.\n'
        'Any future upgrade requires a separate manual review.\n'
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--release', required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--summary')
    args = parser.parse_args()
    release = json.loads(Path(args.release).read_text(encoding='utf-8'))
    report = release_report(release, Path(args.version).read_text(encoding='utf-8'))
    print(report)
    if args.summary:
        with Path(args.summary).open('a', encoding='utf-8') as summary:
            summary.write(report)


if __name__ == '__main__':
    main()
