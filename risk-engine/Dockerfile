FROM python:3.12-slim AS builder
WORKDIR /repo/risk-engine
COPY risk-engine/pyproject.toml .
RUN pip install --no-cache-dir .

FROM python:3.12-slim
WORKDIR /repo/risk-engine
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY risk-engine/ .
COPY ai/ /repo/ai
EXPOSE 8081 50051
CMD ["uvicorn", "risk_engine.main:app", "--host", "0.0.0.0", "--port", "8081"]
