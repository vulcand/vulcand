$(document).ready(function(){
    anchors = $('h1 a.headerlink, h2 a.headerlink, h3 a.headerlink');
    $(window).scroll($.debounce( 250, function(){
        var current_top = $(window).scrollTop();
        for(var i=anchors.length-1;i>=0;i--) {
            var anchor_top = $(anchors[i]).parent()[0].offsetTop;
            if (current_top > anchor_top - 200) {
              var href = $(anchors[i]).attr('href');
              if (window.history && window.history.replaceState) {
                  history.replaceState({},document.title,href)
              }

              if ($('.docs-sidebar ul.current li.current a[href="' + href + '"]').length > 0) {
                  $('.docs-sidebar li').removeClass('active');
                  $('.docs-sidebar ul.current li.current a[href="' + href + '"]').parent().addClass('active');
              }
              else {
                  var nav_link = $('.docs-sidebar ul.current li.current a').filter(function(index){
                      var linktxt = "#" + $(this).text();
                      linktxt = linktxt.replace(/ /g, "-");
                      linktxt = linktxt.toLowerCase();
                      return linktxt == href;
                  });

                  if (nav_link.length > 0) {
                      $('.docs-sidebar li').removeClass('active');
                      nav_link.parent().addClass('active');
                  }
              }
              break;
            }
        }
    }));
});