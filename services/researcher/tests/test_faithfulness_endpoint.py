"""T-M4-003 [RED]: Python POST /faithfulness_check endpoint tests.

REQ-DEEP2-006: Endpoint reuses existing SYN-002 faithfulness logic.
"""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient

from researcher.app import app


@pytest.fixture
def client() -> TestClient:
    return TestClient(app)


class TestFaithfulnessEndpoint:
    """Tests for POST /faithfulness_check endpoint."""

    def test_endpoint_exists(self, client: TestClient) -> None:
        """Endpoint POST /faithfulness_check is registered."""
        resp = client.post(
            "/faithfulness_check",
            json={"text": "test", "citations": [], "docs": []},
        )
        # Should not return 404 (route exists).
        assert resp.status_code != 404

    def test_endpoint_returns_200_on_valid_request(self, client: TestClient) -> None:
        """Valid request returns 200 with faithfulness result."""
        resp = client.post(
            "/faithfulness_check",
            json={
                "text": "This is a cited passage [1].",
                "citations": ["[1] Source A"],
                "docs": ["Source A full text"],
            },
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "uncited_sentences_count" in data
        assert "uncited_sentences" in data

    def test_endpoint_returns_uncited_count_zero_when_all_cited(self, client: TestClient) -> None:
        """All-cited text returns uncited_count == 0."""
        resp = client.post(
            "/faithfulness_check",
            json={
                "text": "Cited statement [1]. Another cited statement [2].",
                "citations": ["[1] Source A", "[2] Source B"],
                "docs": ["Source A full text", "Source B full text"],
            },
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["uncited_sentences_count"] == 0

    def test_endpoint_returns_uncited_count_positive_when_uncited(self, client: TestClient) -> None:
        """Text with uncited sentences returns uncited_count > 0."""
        resp = client.post(
            "/faithfulness_check",
            json={
                "text": "This sentence has no citation. But this one does [1].",
                "citations": ["[1] Source A"],
                "docs": ["Source A full text"],
            },
        )
        assert resp.status_code == 200
        data = resp.json()
        # The first sentence is uncited, so count should be >= 1.
        assert data["uncited_sentences_count"] >= 1

    def test_endpoint_rejects_missing_text(self, client: TestClient) -> None:
        """Missing text field returns 422 (Pydantic validation)."""
        resp = client.post(
            "/faithfulness_check",
            json={"citations": [], "docs": []},
        )
        assert resp.status_code == 422

    def test_endpoint_schema_matches_contract(self, client: TestClient) -> None:
        """Response schema has correct field names."""
        resp = client.post(
            "/faithfulness_check",
            json={"text": "test", "citations": [], "docs": []},
        )
        assert resp.status_code == 200
        data = resp.json()
        # Verify expected field names.
        assert isinstance(data["uncited_sentences_count"], int)
        assert isinstance(data["uncited_sentences"], list)
