"""Tests for provider derivation from the process environment.

The environment is always injected, so no test depends on the developer's real
environment (except the one test that checks the os.environ default, which
controls it explicitly).
"""

import os
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger_provider as provider


def _derive(**env):
    return provider.derive_provider(env)


class TestSelectionVariables(unittest.TestCase):

    def test_empty_environment_is_anthropic(self):
        self.assertEqual("anthropic", provider.derive_provider({}))

    def test_bedrock_alone_is_amazon_bedrock(self):
        self.assertEqual("amazon-bedrock", _derive(CLAUDE_CODE_USE_BEDROCK="1"))

    def test_vertex_alone_is_google_vertex_anthropic(self):
        self.assertEqual("google-vertex-anthropic",
                         _derive(CLAUDE_CODE_USE_VERTEX="1"))

    def test_foundry_alone_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_USE_FOUNDRY="1"))

    def test_anthropic_aws_alone_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_USE_ANTHROPIC_AWS="1"))

    def test_mantle_alone_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_USE_MANTLE="1"))

    def test_bedrock_and_vertex_together_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_USE_BEDROCK="1",
                                  CLAUDE_CODE_USE_VERTEX="1"))

    def test_bedrock_and_foundry_together_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_USE_BEDROCK="1",
                                  CLAUDE_CODE_USE_FOUNDRY="1"))

    def test_non_provider_use_variable_is_ignored(self):
        self.assertEqual("anthropic",
                         _derive(CLAUDE_CODE_USE_POWERSHELL_TOOL="1"))

    def test_non_provider_use_variable_does_not_count_as_second_provider(self):
        self.assertEqual("amazon-bedrock",
                         _derive(CLAUDE_CODE_USE_BEDROCK="1",
                                 CLAUDE_CODE_USE_POWERSHELL_TOOL="1"))


class TestEmptyValuesCountAsUnset(unittest.TestCase):

    def test_empty_bedrock_is_unset(self):
        self.assertEqual("anthropic", _derive(CLAUDE_CODE_USE_BEDROCK=""))

    def test_empty_vertex_is_unset(self):
        self.assertEqual("anthropic", _derive(CLAUDE_CODE_USE_VERTEX=""))

    def test_empty_foundry_is_unset(self):
        self.assertEqual("anthropic", _derive(CLAUDE_CODE_USE_FOUNDRY=""))

    def test_empty_bedrock_does_not_count_as_second_provider(self):
        self.assertEqual("google-vertex-anthropic",
                         _derive(CLAUDE_CODE_USE_BEDROCK="",
                                 CLAUDE_CODE_USE_VERTEX="1"))

    def test_empty_base_url_is_unset(self):
        self.assertEqual("anthropic", _derive(ANTHROPIC_BASE_URL=""))

    def test_empty_managed_by_host_is_unset(self):
        self.assertEqual("anthropic",
                         _derive(CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=""))

    def test_empty_provider_base_url_is_unset(self):
        self.assertEqual("amazon-bedrock",
                         _derive(CLAUDE_CODE_USE_BEDROCK="1",
                                 ANTHROPIC_BEDROCK_BASE_URL=""))


class TestAnthropicBaseUrl(unittest.TestCase):

    def test_default_base_url_is_anthropic(self):
        self.assertEqual("anthropic",
                         _derive(ANTHROPIC_BASE_URL="https://api.anthropic.com"))

    def test_default_base_url_with_trailing_slash_is_anthropic(self):
        self.assertEqual("anthropic",
                         _derive(ANTHROPIC_BASE_URL="https://api.anthropic.com/"))

    def test_default_base_url_with_repeated_trailing_slashes_is_anthropic(self):
        self.assertEqual("anthropic",
                         _derive(ANTHROPIC_BASE_URL="https://api.anthropic.com//"))

    def test_default_base_url_host_case_is_ignored(self):
        self.assertEqual("anthropic",
                         _derive(ANTHROPIC_BASE_URL="HTTPS://API.Anthropic.com"))

    def test_proxy_base_url_is_omitted(self):
        self.assertIsNone(_derive(ANTHROPIC_BASE_URL="https://proxy.example.com"))

    def test_default_host_with_path_is_omitted(self):
        self.assertIsNone(
            _derive(ANTHROPIC_BASE_URL="https://api.anthropic.com/v1"))

    def test_default_host_with_port_is_omitted(self):
        self.assertIsNone(
            _derive(ANTHROPIC_BASE_URL="https://api.anthropic.com:8443"))

    def test_default_host_over_http_is_omitted(self):
        self.assertIsNone(_derive(ANTHROPIC_BASE_URL="http://api.anthropic.com"))

    def test_local_gateway_base_url_is_omitted(self):
        self.assertIsNone(_derive(ANTHROPIC_BASE_URL="http://localhost:4000"))


class TestProviderBaseUrlsAndHostManagement(unittest.TestCase):

    def test_bedrock_with_custom_bedrock_base_url_is_omitted(self):
        self.assertIsNone(_derive(
            CLAUDE_CODE_USE_BEDROCK="1",
            ANTHROPIC_BEDROCK_BASE_URL="https://bedrock-gw.example.com"))

    def test_vertex_with_custom_vertex_base_url_is_omitted(self):
        self.assertIsNone(_derive(
            CLAUDE_CODE_USE_VERTEX="1",
            ANTHROPIC_VERTEX_BASE_URL="https://vertex-gw.example.com"))

    def test_managed_by_host_alone_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST="1"))

    def test_managed_by_host_with_bedrock_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_USE_BEDROCK="1",
                                  CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST="1"))

    def test_managed_by_host_with_vertex_is_omitted(self):
        self.assertIsNone(_derive(CLAUDE_CODE_USE_VERTEX="1",
                                  CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST="1"))


class TestEnvironmentSource(unittest.TestCase):

    def test_defaults_to_process_environment(self):
        with mock.patch.dict(os.environ, {"CLAUDE_CODE_USE_BEDROCK": "1"}):
            os.environ.pop("CLAUDE_CODE_USE_VERTEX", None)
            os.environ.pop("CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST", None)
            os.environ.pop("ANTHROPIC_BEDROCK_BASE_URL", None)
            os.environ.pop("CLAUDE_CODE_USE_FOUNDRY", None)
            os.environ.pop("CLAUDE_CODE_USE_ANTHROPIC_AWS", None)
            os.environ.pop("CLAUDE_CODE_USE_MANTLE", None)
            self.assertEqual("amazon-bedrock", provider.derive_provider())

    def test_injected_environment_ignores_process_environment(self):
        with mock.patch.dict(os.environ, {"CLAUDE_CODE_USE_BEDROCK": "1"}):
            self.assertEqual("anthropic", provider.derive_provider({}))

    def test_none_value_is_treated_as_unset(self):
        self.assertEqual("anthropic", _derive(CLAUDE_CODE_USE_BEDROCK=None))

    def test_never_raises_on_odd_values(self):
        try:
            result = _derive(CLAUDE_CODE_USE_BEDROCK=1, ANTHROPIC_BASE_URL=42)
        except Exception as exc:
            self.fail(f"derive_provider raised: {exc}")
        self.assertIn(result, (None, "anthropic", "amazon-bedrock"))


if __name__ == "__main__":
    unittest.main()
