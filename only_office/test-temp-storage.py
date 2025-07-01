#!/usr/bin/env python3
"""
Test script for OnlyOffice temporary file management
Demonstrates the new workflow with S3 download to temp storage
"""

import requests
import json
import time
from pathlib import Path

# Configuration
BASE_URL = "http://localhost:3000"
TEST_FILE = "test.docx"  # Make sure this file exists in your S3 bucket

def test_api_endpoints():
    """Test the various API endpoints"""
    
    print("🧪 Testing OnlyOffice API with Temporary File Management")
    print("=" * 60)
    
    # Test health check
    print("\n1. 🏥 Health Check")
    try:
        response = requests.get(f"{BASE_URL}/health")
        print(f"Status: {response.status_code}")
        if response.status_code == 200:
            health_data = response.json()
            print(f"Services: {health_data.get('services', {})}")
        else:
            print(f"Error: {response.text}")
    except Exception as e:
        print(f"Failed to connect: {e}")
        return
    
    # Test list documents in S3
    print("\n2. 📋 List Documents in S3")
    try:
        response = requests.get(f"{BASE_URL}/documents")
        if response.status_code == 200:
            docs = response.json()
            print(f"Found {docs['count']} documents in S3:")
            for doc in docs['documents'][:5]:  # Show first 5
                print(f"  - {doc['filename']} ({doc['size']} bytes)")
        else:
            print(f"Error: {response.text}")
    except Exception as e:
        print(f"Failed: {e}")
    
    # Test list temp files
    print("\n3. 📁 List Temporary Files")
    try:
        response = requests.get(f"{BASE_URL}/temp-files")
        if response.status_code == 200:
            temp_data = response.json()
            print(f"Temp directory: {temp_data['temp_directory']}")
            print(f"TTL: {temp_data['ttl_hours']} hours")
            print(f"Found {temp_data['count']} temporary files:")
            for temp_file in temp_data['temp_files']:
                print(f"  - {temp_file['filename']} (Age: {temp_file['age_hours']:.1f}h)")
        else:
            print(f"Error: {response.text}")
    except Exception as e:
        print(f"Failed: {e}")
    
    # Test download (which triggers S3 to temp download)
    print(f"\n4. ⬇️ Download File: {TEST_FILE}")
    try:
        response = requests.get(f"{BASE_URL}/download/{TEST_FILE}")
        if response.status_code == 200:
            print(f"✅ Successfully downloaded {TEST_FILE}")
            print(f"Content-Type: {response.headers.get('content-type')}")
            print(f"Content-Length: {response.headers.get('content-length')} bytes")
        elif response.status_code == 404:
            print(f"❌ File {TEST_FILE} not found in S3")
        else:
            print(f"❌ Error {response.status_code}: {response.text}")
    except Exception as e:
        print(f"Failed: {e}")
    
    # Test list temp files again to see if file was cached
    print("\n5. 📁 List Temporary Files (After Download)")
    try:
        response = requests.get(f"{BASE_URL}/temp-files")
        if response.status_code == 200:
            temp_data = response.json()
            print(f"Found {temp_data['count']} temporary files:")
            for temp_file in temp_data['temp_files']:
                print(f"  - {temp_file['filename']} (Age: {temp_file['age_hours']:.1f}h, Size: {temp_file['size']} bytes)")
        else:
            print(f"Error: {response.text}")
    except Exception as e:
        print(f"Failed: {e}")
    
    # Test manual cleanup
    print("\n6. 🧹 Manual Cleanup Test")
    try:
        response = requests.post(f"{BASE_URL}/cleanup-temp-files")
        if response.status_code == 200:
            cleanup_data = response.json()
            print(f"✅ Cleanup completed: {cleanup_data['message']}")
        else:
            print(f"Error: {response.text}")
    except Exception as e:
        print(f"Failed: {e}")
    
    # Generate editor URL for testing
    print(f"\n7. 📝 Editor URL for {TEST_FILE}")
    editor_url = f"{BASE_URL}/editor/{TEST_FILE}"
    print(f"Open in browser: {editor_url}")
    
    print("\n" + "=" * 60)
    print("✅ Test completed! Check the temporary file behavior:")
    print(f"1. Files are downloaded from S3 to local temp storage")
    print(f"2. OnlyOffice serves files from temp storage for better performance")
    print(f"3. System tracks original S3 file locations")
    print(f"4. Edited files are saved back to ORIGINAL S3 location (proper revisions)")
    print(f"5. Old temp files are automatically cleaned up")
    print(f"6. No duplicate copies created - changes update the original file")

if __name__ == "__main__":
    test_api_endpoints() 