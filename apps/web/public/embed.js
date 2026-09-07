(function () {
  var script = document.currentScript;
  var token = script && script.getAttribute("data-token");
  if (!token) return;
  var src = script.getAttribute("data-src") || (script.src ? new URL(script.src).origin + "/embed/" + token : "/embed/" + token);
  var height = script.getAttribute("data-height") || "640";
  var iframe = document.createElement("iframe");
  iframe.src = src;
  iframe.width = "100%";
  iframe.height = height;
  iframe.setAttribute("frameborder", "0");
  iframe.setAttribute("allowfullscreen", "true");
  iframe.style.border = "0";
  iframe.style.minHeight = height + "px";
  script.parentNode && script.parentNode.insertBefore(iframe, script.nextSibling);
})();
