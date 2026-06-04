from fastapi import FastAPI

app = FastAPI(
    title="SyncSpace",
    version="0.1.0"
)

@app.get("/")
async def root():
    return {
        "app": "SyncSpace",
        "status": "running"
    }