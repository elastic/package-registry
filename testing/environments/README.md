# Usage

Firstly, refresh docker images:

```bash
$ docker-compose -f snapshot.yml pull
```

Run docker containers:

```bash
$ docker-compose -f snapshot.yml -f local.yml up --force-recreate
```

# Bump versions

There is an automation in place to bump the Elastic stack versions to a pinned version.

In case you need to manually bump the existing version in `testing/environments/snapshot.yml` then please run the script
`.ci/bump-stack-version.sh <VERSION> "true"`.

Where `<VERSION>` is the docker image tag without the `-SNAPSHOT`, and `"true"` means to create a git branch.

**NOTE**: If you change the versioning format be sure it's also updated accordingly in `.ci/bump-stack-version.sh`.
