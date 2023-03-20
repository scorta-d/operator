


cd config/samples

yq -i -e ".spec.image=\"docker.io/toolbelt/netcat:2023-01-23\"" apps_v1_helloapp.yaml


echo new: $(yq '.spec.image'  apps_v1_helloapp.yaml)

oc apply -f  apps_v1_helloapp.yaml -n test

cd ../..

watch ./05-verify.sh
