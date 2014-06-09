var TOC = {
    load: function () {
        $('#toc_button').click(TOC.toggle);
    },

    toggle: function () {
        if ($('#sphinxsidebar').toggle().is(':hidden')) {
            $('div.document').css('left', "0px");
            $('toc_button').removeClass("open");
        } else {
            $('div.document').css('left', "281px");
            $('#toc_button').addClass("open");
        }
        return $('#sphinxsidebar');
    }
};

$(function () {
    TOC.load();

    // make header fixed on scrolling
    $(window).scroll(function () {
        if ($(this).scrollTop() >= $('#body').offset().top) {
            $('section.subnav').addClass('subnav-fixed');
            $('section.main').addClass('main-with-subnav');
        } else {
            $('section.subnav').removeClass('subnav-fixed');
            $('section.main').removeClass('main-with-subnav');
        }
    });

});


// monkey patch for text highlighting
highlightText_patched = jQuery.fn.highlightText;
jQuery.fn.highlightText = function () {
    highlightText_patched.apply(this, arguments);

    // go to highlighted text if found
    var highlighted_text = $('.document .highlighted')[0];
    if (highlighted_text) {
        var scrolling_pos = $(highlighted_text).offset().top - $('section.subnav').height() - 10;
        $(window).scrollTop(scrolling_pos);
    }
}
