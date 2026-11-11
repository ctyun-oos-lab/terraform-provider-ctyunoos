resource "ctyun_oos_bucket" "test" {
{{- template "region" }}
  bucket = var.rName

{{- template "tags" . }}
}
