import unittest
from pathlib import Path

from upstream_release_report import release_report


class UpstreamReportTest(unittest.TestCase):
    def test_new_release_is_information_not_merge_failure(self):
        report = release_report({'tag_name': 'v0.2.13', 'html_url': 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.13'}, '0.2.11-mg.1')
        self.assertIn('New upstream version available', report)
        self.assertIn('No merge, push, pull request, or deployment', report)

    def test_equal_or_older_release_is_not_an_upgrade(self):
        for tag in ('v0.2.11', 'v0.2.9'):
            self.assertIn('No newer upstream version', release_report({'tag_name': tag}, '0.2.11-mg.1'))

    def test_invalid_metadata_is_rejected(self):
        for data in ({'tag_name': 'v0.2.13\nmalicious'}, {'tag_name': 'v0.2.13', 'prerelease': True}, {'tag_name': 'v0.2.13', 'html_url': 'https://evil.example/release'}):
            with self.assertRaises(ValueError):
                release_report(data, '0.2.11-mg.1')

    def test_workflow_is_read_only_and_does_not_merge(self):
        workflow = (Path(__file__).resolve().parents[1] / '.github/workflows/upstream-check.yml').read_text()
        self.assertIn('contents: read', workflow)
        for operation in ('contents: write', 'pull-requests: write', 'git merge', 'git push', 'gh pr create', 'git checkout -b'):
            self.assertNotIn(operation, workflow)


if __name__ == '__main__':
    unittest.main()
