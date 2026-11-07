FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt && useradd --uid 10001 --create-home app && mkdir /app/data && chown app:app /app/data
COPY --chown=app:app . .
USER app
ENV HOST=0.0.0.0 PORT=8788 DATA_DIR=/app/data
EXPOSE 8788
CMD ["python", "main.py"]
