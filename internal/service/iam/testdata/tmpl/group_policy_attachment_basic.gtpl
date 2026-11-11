resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

resource "ctyun_iam_group" "test" {
  name = var.rName
}

resource "ctyun_iam_policy" "test" {
  name        = var.rName
  description = "A test policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = [
        "iam:ChangePassword"
      ]
      Effect   = "Allow"
      Resource = "*"
    }]
  })
}