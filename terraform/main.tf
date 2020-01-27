# -------------------------------------------------------------------------------
# Resources
# -------------------------------------------------------------------------------
data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

locals {
  configs = [for c in var.configurations : {
    namespace   = c.namespace
    config_path = "${c.namespace}/${basename(c.config)}"
    state_path  = "${c.namespace}/${basename(c.config)}.state"
  }]
}

resource "aws_s3_bucket" "bucket" {
  bucket        = "${data.aws_caller_identity.current.account_id}-${var.name_prefix}"
  region        = data.aws_region.current.name
  acl           = "private"
  force_destroy = true

  versioning {
    enabled = true
  }

  tags = var.tags
}

resource "aws_s3_bucket_object" "configurations" {
  count  = length(local.configs)
  bucket = aws_s3_bucket.bucket.id
  key    = local.configs[count.index].config_path
  source = var.configurations[count.index].config
  etag   = filemd5(var.configurations[count.index].config)
}

data "archive_file" "lambda" {
  type        = "zip"
  output_path = "${path.module}/sidecred.zip"
  source_file = var.binary_path
}

module "lambda" {
  source  = "telia-oss/lambda/aws"
  version = "3.1.0"

  name_prefix      = var.name_prefix
  filename         = "${path.module}/sidecred.zip"
  handler          = basename(var.binary_path)
  source_code_hash = data.archive_file.lambda.output_base64sha256
  policy           = data.aws_iam_policy_document.lambda.json
  environment      = merge({ SIDECRED_CONFIG_BUCKET = aws_s3_bucket.bucket.id, SIDECRED_STATE_BACKEND = "s3", SIDECRED_S3_BACKEND_BUCKET = aws_s3_bucket.bucket.id }, var.environment)
  tags             = var.tags
}

data "aws_iam_policy_document" "lambda" {
  # Read/write request configuration and state file.
  statement {
    effect = "Allow"

    actions = [
      "*",
    ]

    resources = [
      "${aws_s3_bucket.bucket.arn}/*",
    ]
  }

  # Allow STS provider to assume any role with a sidecred = allow tag.
  statement {
    effect = "Allow"

    actions = [
      "sts:AssumeRole",
    ]

    resources = [
      "arn:aws:iam::*:role/*",
    ]

    condition {
      test     = "StringEquals"
      variable = "iam:ResourceTag/sidecred"
      values   = ["allow"]
    }
  }

  # Read/write SSM Parameters to support a SSM secret store.
  # TODO: Expose the prefix as a variable (needs to match the path template)
  statement {
    effect = "Allow"

    actions = [
      "ssm:PutParameter",
      "ssm:DeleteParameter",
    ]

    resources = [
      "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/sidecred*",
    ]
  }

  # Allow lambda to create a log group/stream
  statement {
    effect = "Allow"

    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]

    resources = [
      "*",
    ]
  }
}

resource "aws_cloudwatch_event_rule" "main" {
  count               = length(local.configs)
  name                = "${local.configs[count.index].namespace}-sidecred-trigger"
  description         = "${local.configs[count.index].namespace} sidecred trigger."
  schedule_expression = "rate(10 minutes)"
  tags                = var.tags
}

resource "aws_cloudwatch_event_target" "main" {
  count = length(local.configs)
  rule  = aws_cloudwatch_event_rule.main[count.index].name
  arn   = module.lambda.arn
  input = jsonencode(local.configs[count.index])
}

resource "aws_lambda_permission" "main" {
  count         = length(local.configs)
  statement_id  = "${local.configs[count.index].namespace}-sidecred-permission"
  function_name = module.lambda.arn
  action        = "lambda:InvokeFunction"
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.main[count.index].arn
}

