resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
{{- template "region" }}
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "ctyun_oos_bucket" "test" {
{{- template "region" }}
  bucket = var.rName

}
