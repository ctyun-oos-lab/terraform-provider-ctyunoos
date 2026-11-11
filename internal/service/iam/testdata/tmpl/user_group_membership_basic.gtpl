resource "ctyun_iam_user_group_membership" "test" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.test.name
}

resource "ctyun_iam_user" "test" {
  name          = var.rName
  force_destroy = true
}

resource "ctyun_iam_group" "test" {
  name = var.rName
}