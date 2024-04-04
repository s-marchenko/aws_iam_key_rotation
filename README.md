# AWS IAM Key Rotation

This tool is designed to manage and rotate AWS IAM keys. It provides functionality to create, update, delete, list, and rotate AWS IAM access keys.

## Installation

This project is written in Go. Ensure you have Go installed on your machine.
To install the tool, clone the repository and build the project:

```bash
git clone https://github.com/<your-github-username>/aws_iam_key_rotation.git
cd aws_iam_key_rotation
go build
```

## Usage of the pre-built binaries
Download the pre-built binaries from the releases page and run the tool.  
For macOS users you have to confirm the security settings to run the binary. 
```bash
# Make the binary executable
chmod 755 aws-iam-rotate-darwin-arm64
# Remove the quarantine attribute
xattr -d com.apple.quarantine aws-iam-rotate-darwin-arm64
# Run the binary
./aws-iam-rotate-darwin-arm64 -h
```

## Usage
Use help to see the available commands:
```bash
./aws_iam_key_rotation -h
```

Add to cron to rotate keys every 30 days (the default parameter, can be adjusted).

## Add to cron
```bash 
# Edit the crontab
crontab -e
# Add the following line to the crontab to run rotation every day at 10:00
0 10 * * * /path/to/aws_iam_key_rotation -rotate >> /path/to/logfile.log 2>&1
```

