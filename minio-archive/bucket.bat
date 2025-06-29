@echo off
echo Creating OnlyOffice bucket in MinIO...

REM Install MinIO client if not already available
docker pull minio/mc

REM Configure MinIO client
docker run --rm --network minio_archive_network minio/mc alias set myminio http://minio-archive:9000 s3phatneglo-archive 0yq5h3to9

REM Create the onlyoffice bucket
docker run --rm --network minio_archive_network minio/mc mb myminio/onlyoffice

REM Set permissions on the bucket
docker run --rm --network minio_archive_network minio/mc anonymous set download myminio/onlyoffice
docker run --rm --network minio_archive_network minio/mc anonymous set upload myminio/onlyoffice

echo Bucket created and permissions set. The bucket should now be accessible.