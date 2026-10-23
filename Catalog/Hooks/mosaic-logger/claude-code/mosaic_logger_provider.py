"""mosaic_logger_provider.py -- Provider ID derivation from the process environment.

The provider is the models.dev identifier of the service that served the
model. It is derived only from environment variables (settings-file `env`
blocks already reach hook processes). When the route cannot be told
unambiguously the provider is None and callers omit the field.
"""

import os
from urllib.parse import urlsplit

PROVIDER_ANTHROPIC = "anthropic"
PROVIDER_AMAZON_BEDROCK = "amazon-bedrock"
PROVIDER_GOOGLE_VERTEX_ANTHROPIC = "google-vertex-anthropic"

ANTHROPIC_DEFAULT_BASE_URL = "https://api.anthropic.com"

_USE_BEDROCK = "CLAUDE_CODE_USE_BEDROCK"
_USE_VERTEX = "CLAUDE_CODE_USE_VERTEX"
_SELECTION_VARS = (
    _USE_BEDROCK,
    _USE_VERTEX,
    "CLAUDE_CODE_USE_FOUNDRY",
    "CLAUDE_CODE_USE_ANTHROPIC_AWS",
    "CLAUDE_CODE_USE_MANTLE",
)


def _is_set(env, name: str) -> bool:
    value = env.get(name)
    return isinstance(value, str) and value != ""


def _normalize_url(url: str) -> str:
    """Strip trailing slashes and lowercase scheme and host; keep the rest."""
    url = url.rstrip("/")
    parts = urlsplit(url)
    if not parts.scheme or not parts.netloc:
        return url
    netloc = parts.netloc.lower() if "@" not in parts.netloc else (
        parts.netloc.rsplit("@", 1)[0] + "@" + parts.netloc.rsplit("@", 1)[1].lower())
    rest = url[len(parts.scheme) + 3 + len(parts.netloc):]
    return parts.scheme.lower() + "://" + netloc + rest


def _is_default_base_url(url: str) -> bool:
    return _normalize_url(url) == _normalize_url(ANTHROPIC_DEFAULT_BASE_URL)


def _derive(env) -> "str | None":
    selected = [n for n in _SELECTION_VARS if _is_set(env, n)]
    if len(selected) > 1:
        return None
    if _is_set(env, "CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST"):
        return None
    if selected == [_USE_BEDROCK]:
        if _is_set(env, "ANTHROPIC_BEDROCK_BASE_URL"):
            return None
        return PROVIDER_AMAZON_BEDROCK
    if selected == [_USE_VERTEX]:
        if _is_set(env, "ANTHROPIC_VERTEX_BASE_URL"):
            return None
        return PROVIDER_GOOGLE_VERTEX_ANTHROPIC
    if selected:
        return None
    if _is_set(env, "ANTHROPIC_BASE_URL") and not _is_default_base_url(
            env["ANTHROPIC_BASE_URL"]):
        return None
    return PROVIDER_ANTHROPIC


def derive_provider(env: "Mapping[str, str] | None" = None) -> "str | None":  # noqa: F821
    """Provider ID for events built by this process, or None when it cannot be
    told unambiguously. env defaults to os.environ. Never raises."""
    try:
        return _derive(os.environ if env is None else env)
    except Exception:
        return None
