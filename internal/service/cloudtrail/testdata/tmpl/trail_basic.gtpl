resource "ctyun_oos_bucket" "test" {
  bucket = var.rName
}

resource "ctyun_cloudtrail" "test" {
  name           = var.rName
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = false
{{- template "tags" . }}
}
