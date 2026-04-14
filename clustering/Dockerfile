FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev gcc g++ libc6-dev curl \
    && ARCH="$(dpkg --print-architecture)" \
    && curl -fsSL "https://github.com/aptible/supercronic/releases/download/v0.2.33/supercronic-linux-${ARCH}" \
    -o /usr/local/bin/supercronic \
    && chmod +x /usr/local/bin/supercronic \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY train_umap.py run_clustering.py crontab ./

CMD ["/usr/local/bin/supercronic", "/app/crontab"]
