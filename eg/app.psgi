#!perl

my $app = sub {
    [200, [], ["plack $ENV{NAME}: $ENV{VALUE}"]]
}