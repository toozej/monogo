# @metadata owner:security contact.email:security@example.com
resource "google_project_iam_binding" "project" {
  project = "your-project-id"
  role    = "roles/editor"

  members = [
    "user:jane@example.com",
  ]
}
