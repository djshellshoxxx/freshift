'use strict';
/* Communicates only with the FreshSift Pages path; fetches happen in extension worker. */
(() => {
  if (location.origin !== 'https://djshellshoxxx.github.io' ||
      !location.pathname.startsWith('/freshift/')) return;
  const notify = message => window.postMessage(message, location.origin);
  window.addEventListener('message', event => {
    if (event.source !== window || event.origin !== location.origin || !event.data) return;
    const data = event.data;
    if (data.freshsift === 'ping') { notify({freshsift:'ready'}); return; }
    if (data.freshsift !== 'request' || !Number.isSafeInteger(data.id)) return;
    chrome.runtime.sendMessage({operation:'freshsift-fetch',
      url:data.url,method:data.method,headers:data.headers,
      allowPrivate:data.allowPrivate,timeoutSeconds:data.timeoutSeconds
    }, result => {
      const error = chrome.runtime.lastError?.message;
      notify({freshsift:'response',id:data.id,
        result:error?{error:'Extension unavailable: '+error}:result||{error:'Empty extension response'}});
    });
  });
  notify({freshsift:'ready'});
})();