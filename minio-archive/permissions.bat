@echo off
echo Setting basic permissions on MinIO bucket...

REM Install MinIO client if not already available
docker pull minio/mc

REM Configure MinIO client
docker run --rm --network minio_archive_network minio/mc alias set myminio http://minio-archive:9000 s3phatneglo-archive 0yq5h3to9

REM Just make the bucket publicly accessible (read/write)
docker run --rm --network minio_archive_network minio/mc anonymous set download myminio/onlyoffice
docker run --rm --network minio_archive_network minio/mc anonymous set upload myminio/onlyoffice

echo Basic MinIO permissions set. The bucket should now be accessible.