from fastapi.testclient import TestClient
from embedder.main import app

client = TestClient(app)


def test_health_returns_200():
    response = client.get("/health")
    assert response.status_code == 200


def test_health_returns_expected_json():
    response = client.get("/health")
    assert response.json() == {"status": "ok", "service": "embedder"}
