$(function(){

  $('a[data-toggle="tab"]').bind('shown', function(e) {
    var currentTab = e.target;
    var tabContent = $($(currentTab).attr('href'));
    var dataUrl = tabContent.data('url');

    $.getJSON(dataUrl).success(function(data){
      var template = $('#dj_reports_template').html();
      if(data.length > 0)
        var output = Mustache.render(template, data);
      else
        var output = "<div class='alert centered'>No Jobs</div>";
      tabContent.html(output);
    });

  })

  $('.nav.nav-tabs li.active a[data-toggle="tab"]').trigger('shown');

  $('a[rel=popover]').live('mouseenter', function(){
    $(this).popover('show');
  });

  $('a[rel=modal]').live('click', function(){
    var template = $($(this).attr('href')).html();
    var output = Mustache.render(template, { content: $(this).data('content') });
    $(output).appendTo($('body')).show();
  });

  $('[data-dismiss="modal"]').live('click', function(){
    $('.modal').hide().remove();
  });

  (function refresh() {
    $.getJSON("status").success(function(data){
      setTimeout(refresh, 5000);

      var template = $('#daemontools_app_template').html();
      var output = Mustache.render(template, data);
      $('#daemontools-app-view').html(output);


      $('form').submit(function(){
          $.ajax({
           url: $(this).attr('action'),
           type:'post',           //数据发送方式
           dataType:'json',      //接受数据格式
           data: {},             //要传递的数据
           complete:function(resp){
            var params = {level: "success",  data: resp.responseText}
            if (resp.status != 200) {
              params.level = "warning"
            }

            var template = $('#daemontools_message_template').html();
            var output = Mustache.render(template, params);
            $('#daemontools-message-view').html(output);
           } //回传函数(这里是函数名)
         });
         return false
      });

    });
  })();

})