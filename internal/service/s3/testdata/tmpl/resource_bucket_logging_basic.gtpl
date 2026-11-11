resource "ctyun_oos_bucket_logging" "test" {
{{- template "region" }}
  bucket = ctyun_oos_bucket.test.bucket

  target_bucket = ctyun_oos_bucket.log_bucket.bucket
  target_prefix = "log/"
}

resource "ctyun_oos_bucket" "log_bucket" {
{{- template "region" }}
  bucket = "${var.rName}-log"
}

resource "ctyun_oos_bucket" "test" {
{{- template "region" }}
  bucket = var.rName
}
