# CoinAPI Custom Server

___
## Setting up Docker
Build the image
```
docker build -t <any_name> .
```
Run the following code on the local machine to run docker container
```
docker run --env-file .env -d -p 8080:8080 <image_name>
```
---
## Reference for setting up on the AWS EC2 Instance
Create tar file for the image to get transported to the EC2 instance
```
docker save -o <name>.tar <image-name>
```
Transport both tar file and env file associated with the program to the EC2 instance
```
scp -i <pem_file> <your_image_file>.tar <ec2-ipaddress>:pphyo
```
ssh into your ec2 instance and run the container
```
ssh -i <pem_file> <ec2_instance>
```
load the transferred tar file
```
docker load -i <tar_file>
```
Run the image as run in the local docker container

