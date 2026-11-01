# Upload storage protection

PixelHop pauses new uploads with HTTP **507** and `Retry-After: 60` if its data filesystem or system temporary filesystem is low on space or cannot be measured. This applies to normal uploads, URL imports, legacy ShareX, share-target uploads, avatars, AI upload tools and every phase of chunk uploads. Existing downloads and deletion remain available.

The default free-space reserve is the greater of **1024 MiB** and **5%** of filesystem capacity. Set `PIXELHOP_MIN_FREE_MB` and `PIXELHOP_MIN_FREE_PERCENT` to positive values to tune it; nonpositive values use the defaults. Admission also budgets three times the incoming data for temporary/processed copies and serialises upload requests within the serving process. It is a capacity guard, not a filesystem quota: other services and background workers can consume space concurrently.

`GET /api/v1/upload-status` returns availability without caching. Upload pages display a warning and disable file selection while unavailable, checking again every 30 seconds. API checks remain authoritative for drag/drop and direct clients. Uploads resume automatically when capacity returns; no service restart is required. S3 installations still check the local data/temp filesystems used for SQLite and processing; this guard does not measure a remote bucket's capacity.

For recovery, inspect disk usage, remove expired/unwanted media through existing tools, check janitor operation or expand storage. Do not remove live SQLite/WAL files. The application does not delete active user content automatically just because the disk is low.
