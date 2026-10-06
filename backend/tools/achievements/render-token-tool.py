"""Render the shared eligibility query into a snapshot or reviewed apply tool."""
import argparse
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('tool', choices=['snapshot', 'apply'])
args = parser.parse_args()
directory = Path(__file__).resolve().parent
template = directory / ('token-snapshot.sql' if args.tool == 'snapshot' else 'historical-token-progress.sql')
text = template.read_text(encoding='utf-8')
if text.count('/*TOKEN_SOURCE_SQL*/') != 1:
    raise ValueError('Expected one eligibility query placeholder')
print(text.replace('/*TOKEN_SOURCE_SQL*/', (directory / 'token-source.sql').read_text(encoding='utf-8')))
