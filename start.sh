echo "Please set the IP address of the cloudApiUrl; and then remove this line" && exit 1; 
helm upgrade --install sample-controller helm/ --create-namespace --set cloudApiUrl="http://10.10.10.10:9001"
