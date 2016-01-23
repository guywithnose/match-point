self.addEventListener('install', function(event) {
  self.skipWaiting();
});

self.addEventListener('push', function(event) {
  event.waitUntil(
    fetch('/notification', {credentials: 'include'}).then(
      function(response) {
        response.json().then(function(data) {
          return self.registration.showNotification(data.title, {'body':data.body});
        });
      }
    )
  );
});

self.addEventListener('notificationclick', function(event) {
  event.notification.close();
  var url = 'https://matchpoint.robertabittle.com';
  event.waitUntil(
    clients.matchAll({
      type: 'window'
    }).then(function(windowClients) {
      for (var i = 0; i < windowClients.length; i++) {
        var client = windowClients[i];
        if (client.url === url && 'focus' in client) {
          return client.focus();
        }
      }
      if (clients.openWindow) {
        return clients.openWindow(url);
      }
    })
  );
});
