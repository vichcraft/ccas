terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region  = var.region
  profile = var.aws_profile
}

# Use the account's default VPC and its subnets — simplest path for a 3-VM experiment.
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# Latest Ubuntu 22.04 LTS amd64 AMI, looked up dynamically per-region.
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_security_group" "ccaas" {
  name        = "ccaas-sg"
  description = "CCaaS three-layer experiment"
  vpc_id      = data.aws_vpc.default.id

  # SSH from the operator's IP only.
  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.ssh_cidr]
  }

  # Storage and CCaaS gRPC ports — only reachable from inside the VPC.
  # The Go servers use insecure credentials, so do NOT widen this.
  ingress {
    description = "CCaaS layers (intra-VPC only)"
    from_port   = 50051
    to_port     = 50052
    protocol    = "tcp"
    cidr_blocks = [data.aws_vpc.default.cidr_block]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Project = "ccaas" }
}

locals {
  roles = ["storage-vm", "ccaas-vm", "bench-vm"]
}

resource "aws_instance" "ccaas" {
  for_each = toset(local.roles)

  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  subnet_id              = data.aws_subnets.default.ids[0]
  vpc_security_group_ids = [aws_security_group.ccaas.id]

  tags = {
    Name    = each.key
    Project = "ccaas"
    Role    = each.key
  }
}
