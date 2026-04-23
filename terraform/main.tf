resource "google_pubsub_schema" "slack_event" {
  name       = "slack-event"
  type       = "PROTOCOL_BUFFER"
  definition = file("../proto/threadops/v1/slack_event.proto")
}

resource "google_pubsub_topic" "slack_events" {
  name = "slack-events"

  schema_settings {
    schema   = google_pubsub_schema.slack_event.id
    encoding = "JSON"
  }
}
