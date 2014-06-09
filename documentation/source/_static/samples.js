var samples = {

    langs: function () {
        if (!this._langs) {
            this._langs = [];
            var self = this;
            $("li.lang a").each(function () {
                self._langs.push($(this).attr("id"));
            });
        }
        return this._langs;
    },

    sections: function () {
        if (!this._sections) {
            this._sections = [];
            var hasAllLangs = "";
            for (var i = 0; i < this.langs().length; i++) {
                hasAllLangs += ":has(div.highlight-" +
                    this.langs()[i] + ")";
            }
            this._sections = $("div.body div.section").children();
        }
        return this._sections;
    },

    hide: function () {
        for (var i = 0; i < this.langs().length; i++) {
            var id = this.langs()[i];
            id = id.replace("lang_", "");
            this.hideSamples(id);
        }
    },

    show: function () {
        this.change($.cookie('samples_code_language') || 'bash');
    },

    change: function (to, from) {
        from = from || this.current;

        // grab random element in body for scrolltop comparison
        var some_elem = $('p:in-viewport')[0];
        if (!some_elem) {
            some_elem = document.elementFromPoint(
            $('div.bodywrapper').offset().left + 20,
            $('section.subnav').height() + 30);
        }
        var old_top = $(some_elem).offset().top;

        this.hide();
        this.showSamples(to || 'bash');

        // calculate change in offset and scroll the difference
        // (for better user experience)
        var new_top = $(some_elem).offset().top;
        var changed_top = new_top - old_top;
        var current_top = $(window).scrollTop();
        $(window).scrollTop(current_top + changed_top);

        var date = new Date();
        date.setDate(date.getDate() + 365);
        $.cookie('samples_code_language', to,
                 { expires: date});
    },

    detectSample: function () {
        // Detect the first sample shown in the screen
        return window.underscore.find(
            $('div.document p + div[class|=highlight][style*=block]'),
            function(el) {
                return $(el).offset().top >= $(window).scrollTop();
            });
    },

    isSampleOutput: function (elem) {
        return $(elem).hasClass('highlight-javascript')
    },

    hasSamplesFor: function (lang) {
        return ($("div.body div.section").children(":has(div.highlight-" + lang + ")").length != 0);
    },

    sourceOnly: function (sample) {
        var next = sample.next();
        while (next.attr("class") != undefined) {
            if (next.attr("class").indexOf("highlight") == -1) {
                next = next.next();
            } else {
                return !this.isSampleOutput(next);
            }
        }
        return true;
    },

    setDisplay: function (id, display) {
        for (var i = 0; i < this.sections().length; i++) {
            var section = $(this.sections()[i]);
            var samples = section.children("div.highlight-" + id);
            var self = this;
            samples.each(function () {
                var sample = $(this);
                sample.css('display', display);
            });
        }
    },

    showSamples: function (id) {
        this.setDisplay(id, 'block');
        var elem = $("li.lang #lang_" + id);
        elem.attr('class', 'current');
        elem.parent().addClass('active');
        this.current = id;
    },

    hideSamples: function (id) {
        this.setDisplay(id, 'none');
        var elem = $("li.lang #lang_" + id);
        elem.removeClass('current');
        elem.parent().removeClass('active');
    },

};

$(function () {
    samples.show();

    // disable language buttons if there are no samples
    underscore.each(samples.langs(), function (lang) {
        if (!$('.highlight-' + lang).length) {
            // $('#' + lang).css({ 'color': '#ccc' });
        }
    });

    $("li.lang a").click(function(e){
        e.preventDefault();
        var to_lang = $(this).attr("id");
        to_lang = to_lang.replace("lang_", "");
        samples.change(to_lang);
    });
});