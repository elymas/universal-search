import importlib
import pytest


@pytest.mark.parametrize("svc", ["researcher", "storm", "embedder"])
def test_service_is_importable(svc):
    importlib.import_module(svc)
