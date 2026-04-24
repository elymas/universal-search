from fastapi import FastAPI

app = FastAPI(title="universal-search-researcher")


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok", "service": "researcher"}
