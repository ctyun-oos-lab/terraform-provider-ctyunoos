resource "ctyun_iam_user" "test" {
  name = var.rName
{{- template "tags" . }}
}
