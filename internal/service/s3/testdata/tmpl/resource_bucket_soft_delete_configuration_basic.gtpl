resource "ctyun_oos_bucket_soft_delete_configuration" "test" {
{{- template "region" }}
  bucket = ctyun_oos_bucket.test.bucket
  days   = 7
}

resource "ctyun_oos_bucket" "test" {
{{- template "region" }}
  bucket = var.rName

}
