resource "ctyun_oos_bucket_policy" "test" {
{{- template "region" }}
  bucket = ctyun_oos_bucket.test.bucket
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "*"
    Statement = [
      {
        Sid       = "*"
        Effect    = "Allow"
        Principal = { CTYUN = "arn:ctyun:iam:::user/test" }
        Action    = "oos:*"
        Resource  = "arn:ctyun:oos:::${var.rName}/*"
      }
    ]
  })
}

resource "ctyun_oos_bucket" "test" {
{{- template "region" }}
  bucket = var.rName

}
