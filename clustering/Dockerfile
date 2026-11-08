FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev gcc g++ libc6-dev ca-certificates curl \
    && ARCH="$(dpkg --print-architecture)" \
    # Official Supercronic v0.2.33 assets; bypass the failing release-download endpoint.
    && case "$ARCH" in \
        386) ASSET_ID=198280788 ;; \
        amd64) ASSET_ID=198280787 ;; \
        arm) ASSET_ID=198280786 ;; \
        arm64) ASSET_ID=198280789 ;; \
        *) exit 1 ;; \
    esac \
    && curl --retry 3 -fsSL -H "Accept: application/octet-stream" \
    "https://api.github.com/repos/aptible/supercronic/releases/assets/${ASSET_ID}" \
    -o /usr/local/bin/supercronic \
    && chmod +x /usr/local/bin/supercronic \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY operations.py vector_rows.py train_umap.py run_clustering.py test_clustering.py test_operations.py test_train_umap.py crontab ./

CMD ["/usr/local/bin/supercronic", "/app/crontab"]
