variable "region" {
  description = "AWS region — keep all three VMs in the same region/AZ for low RTT."
  type        = string
  default     = "us-west-1"
}

variable "aws_profile" {
  description = "Named profile from ~/.aws/credentials to use. Required when multiple profiles exist."
  type        = string
}

variable "instance_type" {
  description = "EC2 instance type. Same type for all three VMs so layer comparisons aren't skewed."
  type        = string
  default     = "t3.medium"
}

variable "key_name" {
  description = "Name of an existing EC2 key pair (already uploaded in the target region) for SSH access."
  type        = string
}

variable "ssh_cidr" {
  description = "CIDR allowed to SSH in. Set to YOUR_PUBLIC_IP/32 — never 0.0.0.0/0."
  type        = string
}
