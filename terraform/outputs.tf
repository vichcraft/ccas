output "storage_private_ip" {
  description = "storage-vm private IP — pass as --storage-addr to ccaas-server and benchmark."
  value       = aws_instance.ccaas["storage-vm"].private_ip
}

output "ccaas_private_ip" {
  description = "ccaas-vm private IP — pass as --ccaas-addr to benchmark."
  value       = aws_instance.ccaas["ccaas-vm"].private_ip
}

output "bench_private_ip" {
  value = aws_instance.ccaas["bench-vm"].private_ip
}

output "ssh_commands" {
  description = "Ready-to-paste SSH commands for each VM."
  value = {
    for role, inst in aws_instance.ccaas :
    role => "ssh ubuntu@${inst.public_ip}"
  }
}

output "scp_targets" {
  description = "Public IPs (for scp uploads from your laptop)."
  value = {
    for role, inst in aws_instance.ccaas :
    role => inst.public_ip
  }
}
