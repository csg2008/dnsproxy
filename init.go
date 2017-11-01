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
		<meta http-equiv="Content-Type" content="text/html; charset=windows-1252">
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
				var name = document.getElementById('name').value;
				var types = document.getElementsByName('type');
				var i;
				var mytype = 255
				for (i = 0; i < types.length; i++) {
					if (types[i].checked == true) {
						mytype = types[i].value;
					}
				}
				query('/',
				function (err, data) {
					if (err != null) {
						alert('Something went wrong: ' + err);
					} else {
						document.getElementById("json").innerHTML = JSON.stringify(data, undefined, 2);
					}
				}, "name=" + name + "&type=" + mytype);
			}
		</script>
	</head>
	
	<body>
		<form>
			<table width=100% bgcolor="#808080">
				<tr>
					<td>DNS Lookup</td>
					<td><input type="text" name="name" id="name"></td>
					<td><input type="button" name="lookup" value="Resolve" OnClick=ResolveName()></td>
				</tr>
			</table>
			<table>
				<tr>
					<td><input type="radio" name="type" id="type-1" value="1"> A </td>
					<td><input type="radio" name="type" id="type-28" value="28"> AAAA </td>
					<td><input type="radio" name="type" id="type-5" value="5"> CNAME </td>
					<td><input type="radio" name="type" id="type-15" value="15"> MX </td>
					<td><input type="radio" name="type" id="type-255" value="255" checked> ANY </td>
				</tr>
			</table>
		</form>
		<br />
		<p> Results:</p>
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
