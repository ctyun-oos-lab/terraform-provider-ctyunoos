resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
{{- template "region" }}
  bucket = ctyun_oos_bucket.test.bucket
  rule {
    id     = var.rName
    prefix = "prefix/"
    status = "Enabled"

    expiration {
      days = 365
    }
  }
}

resource "ctyun_oos_bucket" "test" {
{{- template "region" }}
  bucket = var.rName
}
