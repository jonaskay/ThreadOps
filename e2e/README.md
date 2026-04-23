# E2E

## Getting started

To run the Pub/Sub emulator, run

    $ docker compose up

To run the tests, run

    $ go test

## Pub/Sub schema validation

The e2e tests use the Pub/Sub emulator, which does **not** enforce schema validation. Messages that would be rejected by the live GCP topic (e.g. non-conforming payloads) will pass through the emulator without error. Real schema validation only applies when running against a live GCP topic with the schema attached.
