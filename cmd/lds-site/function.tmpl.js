function handler(event) {
    var request = event.request;
    var headers = request.headers;
    var host = headers.host.value;
    var uri = request.uri;

    // Configuration injected by deployment tool
    var moduleRegistry = {}; // %%MODULES_JSON%%
    var webfingerRegistry = {}; // %%WEBFINGER_JSON%%
    var email = ""; // %%EMAIL%%
    var canonicalHost = ""; // %%CANONICAL_HOST%%

    // 1. Canonical Host Redirect
    if (host !== canonicalHost) {
        return {
            statusCode: 301,
            statusDescription: "Moved Permanently",
            headers: {
                "location": { "value": "https://" + canonicalHost + uri }
            }
        };
    }


    // 2. Webfinger
    if (uri === "/.well-known/webfinger") {
        var query = request.querystring;
        var resource = "";
        if (query.resource && query.resource.value) {
            resource = query.resource.value;
        }

        // Parse resource to get email (remove acct:)
        var targetEmail = resource;
        if (targetEmail.indexOf("acct:") === 0) {
            targetEmail = targetEmail.substring(5);
        }

        var links = webfingerRegistry[targetEmail];

        if (links) {
            var response = {
                subject: "acct:" + targetEmail,
                links: links
            };

            return {
                statusCode: 200,
                statusDescription: "OK",
                headers: {
                    "content-type": { "value": "application/json" }
                },
                body: {
                    encoding: "text",
                    data: JSON.stringify(response)
                }
            };
        }
        
        // If not found, fall through or return 404? 
        // For now, if we don't match, maybe we should return 404.
        return {
            statusCode: 404,
            statusDescription: "Not Found",
             headers: {
                "content-type": { "value": "text/plain" }
            },
            body: {
                encoding: "text",
                data: "Not Found"
            }
        };
    }

    // 3. Go Modules
    // Check if URI matches any module path prefix
    // moduleRegistry keys are like "oauth2ext" (from map key) or I can just iterate values.
    // The registry structure is map[string]ModuleInfo.

    // We need to match against the module path.
    // URI: /oauth2ext/...

    // Sort keys by length desc to match most specific first?
    // In this case, keys are short names.

    for (var key in moduleRegistry) {
        if (Object.prototype.hasOwnProperty.call(moduleRegistry, key)) {
            var mod = moduleRegistry[key];
            var modPath = "/" + key; // e.g. /oauth2ext

            // Check if request is for this module (exact or subpath)
            if (uri === modPath || uri.indexOf(modPath + "/") === 0) {
                var query = request.querystring;
                
                // Determine redirect target
                // Default: pkg.go.dev
                var targetBase = "https://pkg.go.dev/" + mod.Path;
                var isFixed = false;
                
                if (mod.RedirectTo && mod.RedirectTo !== "") {
                    targetBase = mod.RedirectTo;
                    isFixed = true;
                }

                // If go-get=1, return meta tags
                if (query["go-get"] && query["go-get"].value === "1") {
                    var html = '<!DOCTYPE html>';
                    html += '<html lang="en">';
                    html += '<head>';
                    html += '<meta charset="UTF-8">';
                    html += '<meta name="go-import" content="' + mod.Path + ' git ' + mod.GitURL + '">';
                    html += '<meta http-equiv="refresh" content="0; url=' + targetBase + '">';
                    html += '</head>';
                    html += '<body>';
                    html += 'Redirecting to <a href="' + targetBase + '">' + targetBase + '</a>...';
                    html += '</body></html>';

                    return {
                        statusCode: 200,
                        statusDescription: "OK",
                        headers: {
                            "content-type": { "value": "text/html; charset=utf-8" }
                        },
                        body: {
                            encoding: "text",
                            data: html
                        }
                    };
                }

                // Browser Redirect
                var finalTarget = targetBase;
                if (!isFixed) {
                     // If not fixed, append subpath for pkg.go.dev
                     var suffix = uri.substring(modPath.length);
                     finalTarget = targetBase + suffix;
                }

                return {
                    statusCode: 302,
                    statusDescription: "Found",
                    headers: {
                        "location": { "value": finalTarget }
                    }
                };
            }
        }
    }

    return request;
}
