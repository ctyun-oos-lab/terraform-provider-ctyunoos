resource "ctyun_iam_policy" "test" {
  name = var.rName

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = [
        "ec2:Describe*"
      ]
      Effect   = "Allow"
      Resource = "*"
    }]
  })
}