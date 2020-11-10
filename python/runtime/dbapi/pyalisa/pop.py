# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

import base64
import hashlib
import hmac
import urllib

import requests


class Pop(object):
    """Pop is a client for sending http requests to alisa gateway.
    """
    @staticmethod
    def request(url, params, secret, timeout=(30, 120)):
        """Send a request to alisa and return the status
        code and response body

        Args:
            url(string): the url to request
            params(dict[string]string): the params for the request
            secret(string): the secret to use for encrypting
            timeout((int, int)): connect timeout and read timeout

        Returns:
            (int, string) a tuple of status code and response body
        """
        params['Signature'] = Pop.signature(params, 'POST', secret)
        rsp = requests.post(url, params, timeout)
        return rsp.status_code, rsp.text

    @staticmethod
    def signature(params, http_method, secret):
        ''' Calulate signature for params and http_method
        according to https://help.aliyun.com/document_detail/25492.html

        Args:
            params(dict[string]string): the params to signature
            http_method(string): HTTP or HTTPS
            secret(string): the signature secret

        Returns:
            (string) the signature for the given input
        '''
        qry = ""
        for k, v in sorted(params.items()):
            ek = Pop.percent_encode(k)
            ev = Pop.percent_encode(v)
            qry += '&{}={}'.format(ek, ev)
        str = '{}&%2F&{}'.format(http_method, Pop.percent_encode(qry[1:]))
        dig = hmac.new(secret + '&', str, hashlib.sha1).digest()
        return base64.standard_b64encode(dig)

    @staticmethod
    def percent_encode(str):
        """Url param encode, preparing for a pop request.
        c.f. https://help.aliyun.com/document_detail/25492.html

        Args:
            str(string): the param to encode

        Returns:
            (string) the encoded param
        """
        s = urllib.quote_plus(str)
        s = s.replace('+', '%20')
        s = s.replace('*', '%2A')
        return s.replace('%7E', '~')
