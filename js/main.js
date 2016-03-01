"use strict";
$(function() {
  new MatchVm();
});

function MatchVm() {
  var url = location.protocol.replace('http', 'ws') + '//' + location.hostname + ':' + location.port + '/ws',
  socket,
  loginModal = $('#loginModal').modal({show: false}),
  vue = new Vue({
    el: 'body',
    data: function() {
      return {
        activities: [],
        newActivityName: '',
        showAddActivityForm: false,
        isLoggedIn: false,
        userName: '',
        loginPassword: '',
        passwordHash: '',
        loginError: '',
        confirmPassword: '',
        isAdmin: false,
        newUser: false,
        modes: [
          {name: '1v1', numUsers: 2},
          {name: '2v2', numUsers: 4}
        ],
        timer: null
      };
    },
    methods: {
      userJoined: function(activity) {
        for (var i in activity.users) {
          if (activity.users[i].name === this.userName) {
            return true;
          }
        }

        return false;
      },
      userSubscribed: function(activity) {
        for (var i in activity.subscribers) {
          if (activity.subscribers[i] === this.userName) {
            return true;
          }
        }

        return false;
      },
      addActivity: function () {
        this.sendJSON({
          action: 'add-activity',
          newActivity: {
            name: this.newActivityName,
            numUsers: 2,
            users: []
          }
        });
        this.showAddActivityForm = false;
      },
      updateTimeLeft: function() {
        for (var i in this.activities) {
          var now = parseInt((new Date()).getTime() / 1000);
          var secondsLeft = this.activities[i].expires - now;
          if (secondsLeft > 0) {
            var minutes = Math.floor(secondsLeft / 60);
            var seconds = secondsLeft % 60;
            if (seconds < 10) {
              seconds = '0' + seconds;
            }

            Vue.set(this.activities[i], 'timeLeft', minutes + ":" + seconds);
            return;
          }

          Vue.set(this.activities[i], 'timeLeft', null);
        }
      },
      deleteActivity: function (id) {
        this.sendJSON({action: 'delete-activity', activity: {id: id}});
      },
      joinActivity: function (id) {
        this.sendJSON({action: 'join-activity', activity: {id: id}});
      },
      resetActivity: function (id) {
        this.sendJSON({action: 'reset-activity', activity: {id: id}});
      },
      leaveActivity: function (id) {
        this.sendJSON({action: 'leave-activity', activity: {id: id}});
      },
      subscribeActivity: function (id) {
        if ('serviceWorker' in navigator) {
          navigator.serviceWorker.register('notificationWorker.js').then(function() {
            return navigator.serviceWorker.ready;
          }).then(function(reg) {
            reg.pushManager.subscribe({userVisibleOnly: true}).then(function(sub) {
              vue.sendJSON({action: 'add-notify-id', user: {notifyIds: [sub.endpoint.replace(/.*\//, '')]}});
            });
          });
        }

        this.sendJSON({action: 'subscribe-activity', activity: {id: id}});
      },
      unsubscribeActivity: function (id) {
        this.sendJSON({action: 'unsubscribe-activity', activity: {id: id}});
      },
      showLoginForm: function() {
        loginModal.modal('show');
      },
      login: function(salt) {
        this.loginError = "";
        if (this.newUser) {
          if (this.loginPassword !== this.confirmPassword) {
            this.loginError = "Passwords must match.";
            return;
          }

          var salt = getSalt();
          this.passwordHash = sha512(salt + this.loginPassword);
          this.sendJSON({action: 'newUser', user: {salt: salt}});
        } else if (salt) {
          this.passwordHash = sha512(salt + this.loginPassword);
          this.authenticate();
        } else {
          this.sendJSON({action: 'getSalt'});
        }
      },
      authenticate: function() {
        this.sendJSON({action: 'login', user: {name: this.userName, password: this.passwordHash}});
      },
      handleLogin: function(user) {
        localStorage.userName = this.userName = user.name;
        localStorage.passwordHash = this.passwordHash;
        var d = new Date();
        d.setTime(d.getTime() + 2592000000); // 30 days
        document.cookie = "userName=" + this.userName + "; expires=" + d.toUTCString() + "; path=/";
        document.cookie = "passwordHash=" + this.passwordHash + "; expires=" + d.toUTCString() + "; path=/";
        this.isAdmin = user.isAdmin;
        this.isLoggedIn = true;
        this.loginPassword = '';
        loginModal.modal('hide');
      },
      logout: function() {
        this.userName = null;
        this.passwordHash = null;
        delete localStorage.userName;
        delete localStorage.passwordHash;
        this.isLoggedIn = false;
      },
      sendJSON: function(message) {
        if (!message.user) {
          message.user = {
            name: this.userName,
            password: this.passwordHash
          };
        } else {
          message.user.name =  this.userName;
          message.user.password =  this.passwordHash;
        }

        socket.send(JSON.stringify(message));
      },
      selectMode: function(activityId, numUsers) {
        this.sendJSON({action: 'set-numusers', activity: {id: activityId, numUsers: numUsers}});
      },
      formatDate: function(date) {
        var d = new Date(date * 1000);
        var hours = d.getHours();
        return (((hours - 1) % 12) + 1) + ":" + d.getMinutes() + (hours > 12 ? 'PM' : 'AM');
      }
    }
  });

  function initSocket() {
    socket = new WebSocket(url, 'match-point');
    socket.addEventListener("close", function(event) {
      setTimeout(initSocket, 500);
    });

    socket.addEventListener("open", function() {
      this.send(JSON.stringify({action: 'subscribe-all-activities'}));

      if (localStorage.userName) {
        vue.userName = localStorage.userName;
        vue.passwordHash = localStorage.passwordHash;
        vue.authenticate();
      }
    });

    socket.addEventListener("message", handleMessage);
  };

  function handleMessage(event) {
    var message = JSON.parse(event.data);
    if (message.action === 'initialize-activities') {
      if (message.activities) {
        vue.activities = message.activities;
      }
    } else if (message.action === 'update-activity') {
      handleUpdateActivities(message);
    } else if (message.action === 'user-salt') {
      vue.login(message.user.salt);
    } else if (message.action === 'login') {
      vue.handleLogin(message.user);
    } else if (message.action === 'login-error') {
      vue.loginError = message.errorMessage;
    } else if (message.action === 'error') {
      if (message.errorMessage) {
        alert(message.errorMessage);
      }
    }
  }

  function handleUpdateActivities(message) {
    if (message.newActivity.id === "") {
      for (var i in vue.activities) {
        if (vue.activities[i].id == message.oldActivity.id) {
          vue.activities.$remove(vue.activities[i]);
          break;
        }
      }
    } else if (message.oldActivity.id === "") {
      vue.activities.push(message.newActivity);
    } else {
      for (var i in vue.activities) {
        if (vue.activities[i].id === message.newActivity.id) {
          vue.activities.$set(i, message.newActivity);
          break;
        }
      }
    }

    vue.updateTimeLeft();
  }

  if (vue.timer === null) {
    vue.timer = setInterval(vue.updateTimeLeft, 1000);
  }

  initSocket();
}

function getSalt() {
  if (window.crypto) {
    var values = new Uint32Array(30);
    crypto.getRandomValues(values);
    return sha512(values.join(''));
  } else {
    var salt = '';
    for (var i = 0; i < 10; i++) {
      salt += Math.random();
    }

    return sha512(salt);
  }
}
