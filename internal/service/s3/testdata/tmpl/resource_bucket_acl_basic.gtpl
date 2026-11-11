resource "ctyun_oos_bucket_acl" "test" {
{{- template "region" }}
  bucket = ctyun_oos_bucket.test.bucket
  acl    = "private"
}

resource "ctyun_oos_bucket" "test" {
{{- template "region" }}
  bucket = var.rName
}
