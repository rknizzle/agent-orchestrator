# deploy/main.tf
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-east-1" # Change to your preferred region
}

variable "github_token" {
  description = "GitHub Personal Access Token"
  type        = string
  sensitive   = true
}

# Find the latest Amazon Linux 2023 AMI for ARM64
data "aws_ami" "amazon_linux_2023" {
  most_recent = true
  owners      = ["amazon"]
  filter {
    name   = "name"
    values = ["al2023-ami-2023*-arm64"]
  }
}

# Security Group - No incoming traffic needed for a poll-based orchestrator
resource "aws_security_group" "orchestrator_sg" {
  name        = "orchestrator-sg"
  description = "Security group for agent-orchestrator"

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "orchestrator_server" {
  ami           = data.aws_ami.amazon_linux_2023.id
  instance_type = "t4g.micro" # Faster, ARM-based Free Tier

  vpc_security_group_ids = [aws_security_group.orchestrator_sg.id]

  # Pass the setup script as User Data
  user_data = templatefile("${path.module}/setup.sh", {
    github_token = var.github_token
  })

  user_data_replace_on_change = true

  tags = {
    Name = "Agent-Orchestrator-24-7"
  }
}

output "instance_id" {
  value = aws_instance.orchestrator_server.id
}
