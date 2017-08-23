'''
Description:
    This script aims to base64 input data

Input: binary data

Output: base64ed data
'''
import base64
import sys

if __name__ == '__main__':
     try:
         img = sys.stdin.buffer.read()
         base64ed = base64.b64encode(img).decode()
         print(base64ed, end='')
     except Exception as e:
         err_msg = 'python script base64 img data failed: {}'.format(e)
         print(e, file=sys.stderr)
         sys.exit(1)
