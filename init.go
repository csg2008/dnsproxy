package main

import "errors"

// provider dns proxy server handle
var provider map[string]ProxyHandle

// ErrNotFound dns cache key not found
var ErrNotFound = errors.New("cache key is not found")

// ErrCacheTimeout dns cache timeout
var ErrCacheTimeout = errors.New("remote cache timeout")

var page = []byte(`
	<html lang=en>
	<head>
		<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
		<title>HTTP(s) DNS lookup</title>
		<script language="JavaScript">
			// http://stackoverflow.com/questions/12460378/how-to-get-json-from-url-in-javascript
			var query = function (url, callback, data) {
				var xhr = new XMLHttpRequest();
				xhr.open('POST', url, true);
				xhr.setRequestHeader("Content-type","application/x-www-form-urlencoded");
				xhr.responseType = 'json';
				xhr.onload = function () {
					var status = xhr.status;
					if (status == 200) {
						callback(null, xhr.response);
					} else {
						callback(status);
					}
				};
				xhr.send(data);
			};
			function ResolveName() {
				var i;
				var category = 255
				var types = document.getElementsByName('type');
				var hosts = document.getElementById('hosts').value;
				for (i = 0; i < types.length; i++) {
					if (types[i].checked == true) {
						category = types[i].value;
					}
				}
				query('/', function (err, data) {
					if (err != null) {
						alert('Something went wrong: ' + err);
					} else {
						document.getElementById("json").innerHTML = JSON.stringify(data, undefined, 2);
					}
				}, "hosts=" + hosts + "&type=" + category);
			}
		</script>
	</head>
	
	<body>
		<form>
			<div style="background:#808080;size:10px;">
				<span>DNS Lookup</span>
				<span><input type="text" name="hosts" id="hosts" style="width:200px;"></td>
				<span><input type="button" name="lookup" value="Resolve" OnClick=ResolveName()></span>
			</div>
			<div>
				<label><input type="radio" name="type" id="type-1" value="1"> A </label>
				<label><input type="radio" name="type" id="type-28" value="28"> AAAA </label>
				<label><input type="radio" name="type" id="type-5" value="5"> CNAME </label>
				<label><input type="radio" name="type" id="type-15" value="15"> MX </label>
				<label><input type="radio" name="type" id="type-2" value="2"> NS </label>
				<label><input type="radio" name="type" id="type-12" value="12"> PTR </label>
				<label><input type="radio" name="type" id="type-6" value="6"> SOA </label>
				<label><input type="radio" name="type" id="type-16" value="16"> TXT </label>
				<label><input type="radio" name="type" id="type-255" value="255" checked> ANY </label>
			</div>
		</form>
		<br />
		<pre id="json"></pre>
	</body>
	
	</html>
`)

func init() {
	provider = map[string]ProxyHandle{
		"http": NewHTTPServer,
		"raw":  NewNameServer,
	}
}
