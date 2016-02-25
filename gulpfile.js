"use strict"
var gulp = require('gulp'),
plugins = require("gulp-load-plugins")({
  pattern: ['gulp-*', 'main-bower-files'],
  replaceString: /\bgulp[\-.]/
}),
dest = 'public/',
go;

gulp.task('bower', function() {
    return plugins.bower().pipe(gulp.dest('./bower_components'));
});

gulp.task('js', ['bower'], function() {
  var jsFiles = ['js/*.js'];

  gulp.src(plugins.mainBowerFiles().concat(jsFiles))
    .pipe(plugins.filter('*.js'))
    .pipe(plugins.concat('main.js'))
    .pipe(plugins.uglify())
    .pipe(gulp.dest(dest + 'js'));
});

gulp.task('css', ['bower'], function() {
  var cssFiles = ['css/*'];

  gulp.src(plugins.mainBowerFiles().concat(cssFiles))
    .pipe(plugins.filter('*.less'))
    .pipe(plugins.less())
    .pipe(plugins.concat('main.css'))
    .pipe(plugins.cssmin())
    .pipe(gulp.dest(dest + 'css'));
});

gulp.task('fonts', ['bower'], function() {
  var fontFiles = [
    'bower_components/bootstrap/fonts/*',
    'bower_components/font-awesome/fonts/*'
  ];
  gulp.src(fontFiles)
    .pipe(gulp.dest(dest + 'fonts'));
});

gulp.task("go-run", function() {
  go = plugins.go.run("src/main.go", ["src/socket.go"], {cwd: __dirname, stdio: 'inherit'});
});

gulp.task('dev', ['go-run'], function() {
  gulp.watch('js/*.js', ['js']);
  gulp.watch('css/*.less', ['css']);
  gulp.watch('src/*.go').on('change', function() {
    go.restart();
  });;
});

gulp.task('default', ['js', 'css', 'fonts']);
