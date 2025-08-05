# Storage Indexers used in Elastic Package Registry

Elastic Package Registry (EPR) supports multiple ways to retrieve package information. By default, it uses the File system indexer to read packages (folders or zip files).

EPR also provides additional indexers (called storage indexers) that work with Package Storage V2 (technical preview). There are two storage indexer available:
- [Storage indexer](#storage-indexer)
- [SQL storage indexer](#sql-storage-indexer)

You can test and debug these storage indexers locally by following the steps in [this section](#how-to-test-storage-indexers).

## Storage Indexer

**Technical Preview**: This indexer is currently in technical preview. Its behavior and configuration options may change in future releases without prior notice.

In this indexer, package information from a remote index (Package Storage V2) is loaded and kept in memory. The storage indexer periodically checks for updates and refreshes the package data whenever a new index is available.

To enable this indexer, it is required to set these flags (or the corresponding environment variables):
- `feature-storage-indexer`
- `storage-indexer-bucket-internal`
- `storage-endpoint` (optional)
- `storage-indexer-watch-interval` (optional)


## SQL Storage Indexer

**Technical Preview**: This indexer is currently in technical preview. Its behavior and configuration options may change in future releases without prior notice.

In this indexer, package information from a remote index (Package Storage V2) is stored in a SQLite database instead of memory. This approach helps reduce memory usage and aims to make it easier to manage large sets of package data.

Just like the regular storage indexer, the SQL storage indexer periodically updates the database with the latest package information, ensuring your data stays current.

To enable this indexer, it is required to set these flags (or the corresponding environment variables):
- `feature-sql-storage-indexer`
- `storage-indexer-bucket-internal`
- `feature-enable-search-cache` (optional)
- `storage-endpoint` (optional)
- `storage-indexer-watch-interval` (optional)

In addition to the required flags, you can fine-tune the SQL storage indexer’s performance using the following environment variables:

- `EPR_SQL_INDEXER_READ_PACKAGES_BATCH_SIZE`: Sets how many packages are read from storage in each batch (default: `2000`).
  The greater this value, the greater the memory required to load all packages at once.
- `EPR_SQL_INDEXER_DB_INSERT_BATCH_SIZE`: Sets how many packages are inserted into the database per batch (default: `500`).
  Larger batches could speed up database updates by grouping inserts. This value should not be set higher than `EPR_SQL_INDEXER_READ_PACKAGES_BATCH_SIZE`, since you cannot insert more packages than are read in each batch.
- `EPR_SQL_INDEXER_SEARCH_CACHE_SIZE`: Limits the number of items stored in the `/search` endpoint cache (default: `100`).
- `EPR_SQL_INDEXER_SEARCH_CACHE_TTL`: Sets how long cached items remain valid for the `/search` endpoint (default: 10m).
- `EPR_SQL_INDEXER_DATABASE_FOLDER_PATH`:: Specifies the folder path for database creation (default: tmp).

Adjust these variables as needed to optimize memory usage, database performance, and cache behavior for your environment.


## How to test storage indexers

To be able to test these storage indexers, it is required to run the a `fake-gcs` server locally and
configure EPR to use the local address to get the actual index.

Following sections describe the steps to start the services locally as well as how to update the index in the `fake-gcs` server.
This allows us to test and debug the process that runs periodically to read the index and update the required information.

#### Start services locally
Storage indexers can be tested locally following in two different ways:
1. Running Package Registry and a fake GCS server as independent services:
    1. Launch the fake GCS server in one terminal:
        - It creates a new folder with the expected contents for the bucket.
        - It manages a docker-compose scenario with the fake GCS server.
        - The search index JSON file can be downloaded from the [internal CI](https://buildkite.com/elastic/package-storage-infra-indexing/builds?branch=main) and set that file via `-i` parameter.
       ```shell
       cd /path/to/repo/package-registry/
       cd dev
       bash launch_fake_gcs_server.sh -i ../storage/testdata/search-index-all-full.json -b example -c 1
       ```
    2. Tune the configuration used by Package Registry as you require:
        - By default, it uses the `config.yml` file at the root of the repository.
    3. Launch EPR service in a different terminal:
        - It builds package-registry with the contents of the working copy.
        - It triggers the EPR service with the required environment variables to use storage indexers.
       ```shell
       cd /path/to/repo/package-registry/
       cd dev
       bash launch_epr_service_storage_indexer.sh -p ../config.yml
       ```
    4. To stop both services, you just need to press `CTRL+Z` on each terminal. The scripts also manage the cleanup process.
2. Running just Package Registry (fake GCS server runs using the golang library within Package Registry service):
    1. Tune the configuration used by Package Registry as you require:
        - By default, it uses the `config.yml` file at the root of the repository.
    2. Launch EPR service:
        - It builds package-registry with the contents of the working copy.
        - It triggers the EPR service with the required environment variables to use storage indexers.
            - Fake GCS server will run as part of the same process via the Golang library.
       ```shell
       cd /path/to/repo/package-registry/
       cd dev
       bash launch_epr_service_storage_indexer.sh -i ../storage/testdata/search-index-all-full.json -p ../config.yml
       ```
    3. To stop these services, you just need to press `CTRL+Z` on this terminal.

Following these steps, EPR service should be reading files from the storage indexer and there should be log messages like these ones:
```json
{"log.level":"info","@timestamp":"2024-05-27T20:03:35.489+0200","log.origin":{"function":"github.com/elastic/package-registry/storage.(*Indexer).updateIndex","file.name":"storage/indexer.go","file.line":181},"message":"cursor will be updated","cursor.current":"","cursor.next":"1","ecs.version":"1.6.0"}
{"log.level":"info","@timestamp":"2024-05-27T20:03:35.827+0200","log.origin":{"function":"github.com/elastic/package-registry/storage.(*Indexer).updateIndex","file.name":"storage/indexer.go","file.line":192},"message":"Downloaded new search-index-all index","index.packages.size":"1133","ecs.version":"1.6.0"}
```

Package registry service is available at `http://localhost:8080`. Example of query using `curl`:
```shell
curl -s "http://localhost:8080/search"
```

#### Update indices locally

Following the steps mentioned above, a fake GCS server with the contents bucket is going to be
available at http://localhost:4443.

You can query directly to check if the contents of the bucket are available:
    - `http://localhost:4443/storage/v1/b/<bucket>/o`
    - Example:
```shell
 $ curl  -s http://localhost:4443/storage/v1/b/fake-package-storage-internal/o |jq -r .
{
  "kind": "storage#objects",
  "items": [
    {
      "kind": "storage#object",
      "name": "v2/metadata/1/search-index-all.json",
      "id": "fake-package-storage-internal/v2/metadata/1/search-index-all.json",
      "bucket": "fake-package-storage-internal",
      "size": "22157319",
      "crc32c": "s7tNlg==",
      "md5Hash": "49AZWmusj2eLpjH98ePdCw==",
      "etag": "49AZWmusj2eLpjH98ePdCw==",
      "storageClass": "STANDARD",
      "timeCreated": "2025-06-11T09:54:21.239848+02:00",
      "timeStorageClassUpdated": "2025-06-11T09:54:21.239848+02:00",
      "updated": "2025-06-11T09:54:21.239848+02:00",
      "generation": "1749628461332535",
      "selfLink": "/storage/v1/b/fake-package-storage-internal/o/v2%2Fmetadata%2F1%2Fsearch-index-all.json",
      "mediaLink": "/download/storage/v1/b/fake-package-storage-internal/o/v2%2Fmetadata%2F1%2Fsearch-index-all.json?alt=media",
      "metageneration": "1"
    },
    {
      "kind": "storage#object",
      "name": "v2/metadata/cursor.json",
      "id": "fake-package-storage-internal/v2/metadata/cursor.json",
      "bucket": "fake-package-storage-internal",
      "size": "15",
      "crc32c": "oP5v4Q==",
      "md5Hash": "vxi+8LZ201P+UukYETdzvQ==",
      "etag": "vxi+8LZ201P+UukYETdzvQ==",
      "storageClass": "STANDARD",
      "timeCreated": "2025-06-11T09:54:21.239843+02:00",
      "timeStorageClassUpdated": "2025-06-11T09:54:21.239847+02:00",
      "updated": "2025-06-11T09:54:21.239847+02:00",
      "generation": "1749628461239867",
      "selfLink": "/storage/v1/b/fake-package-storage-internal/o/v2%2Fmetadata%2Fcursor.json",
      "mediaLink": "/download/storage/v1/b/fake-package-storage-internal/o/v2%2Fmetadata%2Fcursor.json?alt=media",
      "metageneration": "1"
    }
  ]
}
```

As this URL is available, it can also be updated the contents of the bucket to test the update index process
in the Elastic Package Registry service locally. For that you need to follow these steps:
1. Check the current cursor set. In EPR logs should be a log like this:
   ```json
   {"log.level":"info","@timestamp":"2025-06-11T10:02:17.680+0200","log.origin":{"function":"github.com/elastic/package-registry/storage.(*Indexer).updateIndex","file.name":"storage/indexer.go","file.line":178},"message":"cursor is up-to-date","cursor.current":"1","ecs.version":"1.6.0"}
   ```
2. Update the new index JSON file to the new path `v2/metadata/<cursor>/search-index-all.json` and also update `cursor.json` file in the fake GCS server using the script helper:
   ```shell
   # bash dev/updateIndexDatabase.sh -i <path_to_index> -c <cursor_string> -b <bucket>
   # Example adding a new index using as cursor "2":
   bash dev/updateIndexDatabase.sh -i ./search-index-all.json -c 2 -b fake-package-storage-internal
   ```

After following these steps, the next log messages must appear in the EPR service:
```json
{"log.level":"info","@timestamp":"2025-06-11T10:07:17.680+0200","log.origin":{"function":"github.com/elastic/package-registry/storage.(*Indexer).updateIndex","file.name":"storage/indexer.go","file.line":181},"message":"cursor will be updated","cursor.current":"1","cursor.next":"2","ecs.version":"1.6.0"}
{"log.level":"info","@timestamp":"2025-06-11T10:07:25.749+0200","log.origin":{"function":"github.com/elastic/package-registry/storage.(*Indexer).updateIndex","file.name":"storage/indexer.go","file.line":192},"message":"Downloaded new search-index-all index","index.packages.size":"10775","ecs.version":"1.6.0"}
```
