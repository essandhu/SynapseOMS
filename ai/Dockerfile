FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY smart_router_ml/ smart_router_ml/

ENV MODEL_PATH=/app/model.json

EXPOSE 8090

CMD ["uvicorn", "smart_router_ml.model:app", "--host", "0.0.0.0", "--port", "8090"]
