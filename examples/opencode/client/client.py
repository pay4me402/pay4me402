import os
import sys
import json
import requests

TARGET_URL = os.getenv("TARGET_URL", "https://x402.goplausible.xyz/examples/weather")

def setup_ca_cert():
    certs_path = "certs.json"
    if os.path.exists(certs_path):
        try:
            with open(certs_path, "r") as f:
                data = json.load(f)
            ca_cert_content = data.get("caCert")
            if ca_cert_content:
                ca_path = "/tmp/ca.crt"
                with open(ca_path, "w") as out_f:
                    out_f.write(ca_cert_content)
                # Globally register the CA bundle for python requests and curl
                os.environ["REQUESTS_CA_BUNDLE"] = ca_path
                os.environ["CURL_CA_BUNDLE"] = ca_path
                print(f"Successfully extracted and trusted CA cert from {certs_path}")
        except Exception as e:
            print(f"Warning: Failed to extract CA cert: {e}")

def main():
    setup_ca_cert()
    
    print(f"Sending transparently-proxied request to {TARGET_URL}")
    try:
        # Standard request with full SSL verification! (No verify=False, no warning suppression)
        response = requests.get(TARGET_URL, timeout=30)
        print(f"Status: {response.status_code}")
        print(f"Response: {response.text[:1000]}")
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()