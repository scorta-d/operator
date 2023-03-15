


cd config/samples

yq -i -e ".spec.size=2" apps_v1_helloapp.yaml

echo new size: $(yq '.spec.size'  apps_v1_helloapp.yaml)

oc apply -f  apps_v1_helloapp.yaml

cd ../..

watch ./05-verify.sh
