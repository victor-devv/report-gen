resource "aws_s3_bucket" "reports-bucket" {
  bucket = var.s3_bucket
}

resource "aws_sqs_queue" "reports-queue" {
  name                      = var.sqs_queue
  delay_seconds             = 5
  max_message_size          = 2048
  message_retention_seconds = 86400
  receive_wait_time_seconds = 10
}
